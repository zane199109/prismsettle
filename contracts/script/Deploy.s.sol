// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script, console} from "forge-std/Script.sol";
import {PrismSettleRegistry} from "../src/PrismSettleRegistry.sol";
import {PrismSettleJob} from "../src/PrismSettleJob.sol";
import {ArbitrationHook} from "../src/ArbitrationHook.sol";
import {MockERC20} from "../src/mocks/MockERC20.sol";

/// @title Deploy
/// @notice Phase 3 部署脚本：MockERC20 → Registry → Hook → Job →
///         grantRoles → Seed 4 个官方 Agent → 注册仲裁池。
/// @dev    evaluator 地址来源：
///           - 优先读环境变量 EVALUATOR_ADDRESS（Phase 4 dev.yaml 推导后导出）
///           - 未设置时回退到 msg.sender（dev/anvil 场景，部署者兼任）
///         evaluator 私钥来源：
///           - 读环境变量 PRISM_EVALUATOR_KEY（用于注册仲裁方）
///           - 未设置时跳过仲裁池注册（dev/anvil 场景手动注册）
///         facilitator 地址来源：
///           - 读环境变量 FACILITATOR_ADDRESS，未设置则 address(0)
///           - 真实 x402 Facilitator 由 Phase 9 上线时替换
///         Validator 质押（可选）：
///           - 读 VALIDATOR_ADDRESS + VALIDATOR_STAKE_WEI，未设置则跳过
contract Deploy is Script {
    // Announcement period after dispute resolution (seconds). 20s keeps the
    // demo arbitration flow inside the 3-minute window; production can raise.
    uint256 internal constant ANNOUNCEMENT_PERIOD = 20;

    // 4 个官方 Agent 的 agentId（Seed Phase，对应 PRD FR-C09）
    // AGENT_AUDITOR_SENIOR (0x3333) 兼具官方 Agent 和 Worker 双重身份，声誉 0.9e18
    uint256 internal constant AGENT_DEFI = 0x1111;
    uint256 internal constant AGENT_LABELING = 0x2222;
    uint256 internal constant AGENT_AUDITOR_SENIOR = 0x3333;
    uint256 internal constant AGENT_EVAL = 0x4444;

    // 2 个附加审计 Worker Agent 的 agentId（Phase 8 演示，按声誉竞争抢单）
    uint256 internal constant AGENT_AUDITOR_JUNIOR = 0x7777;
    uint256 internal constant AGENT_AUDITOR_ROOKIE = 0x8888;

    // 官方 Agent 初始声誉分 0.7e18
    uint96 internal constant SEED_SCORE = 0.7e18;

    // 4 个 Agent 的 metadata（endpointUrl + capabilities，JSON 字符串）
    // 实际生产由 offchain/config/agents.yaml 注入，这里用 dev 默认值
    string internal constant META_DEFI = '{"endpointUrl":"http://localhost:9101/invoke","capabilities":"defi"}';
    string internal constant META_LABELING = '{"endpointUrl":"http://localhost:9102/invoke","capabilities":"labeling"}';
    string internal constant META_AUDITOR_SENIOR =
        '{"endpointUrl":"http://localhost:9105/invoke","capabilities":"smart_contract_audit"}';
    string internal constant META_EVAL = '{"endpointUrl":"http://localhost:9104/invoke","capabilities":"evaluation"}';
    string internal constant META_AUDITOR_JUNIOR =
        '{"endpointUrl":"http://localhost:9106/invoke","capabilities":"smart_contract_audit"}';
    string internal constant META_AUDITOR_ROOKIE =
        '{"endpointUrl":"http://localhost:9107/invoke","capabilities":"smart_contract_audit"}';

    function run() external {
        // 读取环境变量（未设置则回退）
        address evaluator = vm.envOr("EVALUATOR_ADDRESS", msg.sender);
        address facilitator = vm.envOr("FACILITATOR_ADDRESS", address(0));
        address validator = vm.envOr("VALIDATOR_ADDRESS", address(0));
        uint256 validatorStake = vm.envOr("VALIDATOR_STAKE_WEI", uint256(0));

        vm.startBroadcast();

        // 1. MockERC20（dev/testnet 用；prod 替换为真实 USDC）
        MockERC20 token = new MockERC20("MockUSDC", "USDC");

        // 2. Registry
        PrismSettleRegistry registry = new PrismSettleRegistry();

        // 3. ArbitrationHook（先部署，jobContract 后置 setJobContract）
        ArbitrationHook hook = new ArbitrationHook(address(token));

        // 4. PrismSettleJob（注入 token + facilitator）
        PrismSettleJob job = new PrismSettleJob(address(token), facilitator, ANNOUNCEMENT_PERIOD);

        // 5. Hook 反向引用：setJobContract + setRegistryContract（一次性）
        hook.setJobContract(address(job));
        hook.setRegistryContract(address(registry));

        // 5b. Job 注册 Registry 地址（用于 grabJob 声誉检查）
        job.setRegistry(address(registry));

        // 5c. Registry 白名单：允许 Job 合约调用 submitValidation(source=3)
        //     （Buyer 评分渠道，complete() 内部依赖）
        registry.setTrustedJob(address(job), true);

        // 6. 角色授权
        registry.grantRole(registry.REGISTRY_EVALUATOR_ROLE(), evaluator);
        job.grantRole(job.COMMERCE_EVALUATOR_ROLE(), evaluator);
        hook.grantRole(hook.RESOLVER_ROLE(), evaluator);

        // 7. Seed Phase：注册 + 设置初始声誉分
        //    注意：registerAgent 由 msg.sender 调用，所有权归 msg.sender。
        //    4 个官方 Agent 的 owner 是部署者（admin），便于后续运维。
        registry.registerAgent(AGENT_DEFI, META_DEFI);
        registry.registerAgent(AGENT_LABELING, META_LABELING);
        registry.registerAgent(AGENT_AUDITOR_SENIOR, META_AUDITOR_SENIOR);
        registry.registerAgent(AGENT_EVAL, META_EVAL);

        registry.seedAgent(AGENT_DEFI, SEED_SCORE);
        registry.seedAgent(AGENT_LABELING, SEED_SCORE);
        registry.seedAgent(AGENT_AUDITOR_SENIOR, 0.9e18); // senior auditor starts with 0.9e18
        registry.seedAgent(AGENT_EVAL, SEED_SCORE);

        // 8. 注册 Provider Agent（0x5555），用于抢单流程
        registry.registerAgent(0x5555, '{"endpointUrl":"","capabilities":"provider"}');
        registry.seedAgent(0x5555, uint96(0.7e18));

        // 9. 注册 2 个附加审计 Worker Agent（Phase 8 演示，按声誉竞争抢单）
        //    senior (0x3333) 已在第 7 步注册，score 0.9e18。
        //    junior (0x7777) score 0.6e18, rookie (0x8888) score 0.3e18。
        registry.registerAgent(AGENT_AUDITOR_JUNIOR, META_AUDITOR_JUNIOR);
        registry.registerAgent(AGENT_AUDITOR_ROOKIE, META_AUDITOR_ROOKIE);
        registry.seedAgent(AGENT_AUDITOR_JUNIOR, 0.6e18);
        registry.seedAgent(AGENT_AUDITOR_ROOKIE, 0.3e18);

        // 10. Validator 质押（可选，仅在 VALIDATOR_ADDRESS 设置时执行）
        if (validator != address(0) && validatorStake > 0) {
            // 注意：stake() 是 payable，需 validator 账户发起并携带 value。
            // 部署脚本无法替 validator 转账；这里仅校验配置，实际质押由
            // validator 自己调用 stake{value: validatorStake}()。
            // 此分支保留为占位，供后续脚本扩展（如 anvil 自动质押）。
        }

        vm.stopBroadcast();

        // 11. 注册仲裁方：以 evaluator 身份 registeArbitrator
        //    使用 PRISM_EVALUATOR_KEY 私钥签名（仅在测试网部署时执行）
        //    注意：vm.envOr 直接读 uint256 无法解析 hex 格式，
        //    改为读 string + 补 0x 前缀 + parseUint。
        string memory evalPkStr = vm.envOr("PRISM_EVALUATOR_KEY", string(""));
        if (bytes(evalPkStr).length > 0) {
            uint256 evalPk = vm.parseUint(string(abi.encodePacked("0x", evalPkStr)));
            vm.startBroadcast(evalPk);
            // 费率 5%（500 bps），收款地址为 evaluator 本人
            hook.registerArbitrator(AGENT_EVAL, 500, evaluator);
            vm.stopBroadcast();
        }

        // 控制台输出（forge script 默认会显示部署地址）
        console.log("MockERC20:", address(token));
        console.log("Registry:", address(registry));
        console.log("ArbitrationHook:", address(hook));
        console.log("PrismSettleJob:", address(job));
        console.log("Evaluator:", evaluator);
        console.log("Facilitator:", facilitator);
    }
}
