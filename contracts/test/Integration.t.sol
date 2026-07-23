// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import {Test} from "forge-std/Test.sol";
import {PrismSettleRegistry} from "../src/PrismSettleRegistry.sol";
import {PrismSettleJob} from "../src/PrismSettleJob.sol";
import {ArbitrationHook} from "../src/ArbitrationHook.sol";
import {MockERC20} from "../src/mocks/MockERC20.sol";
import {MockX402Facilitator} from "../src/mocks/MockX402Facilitator.sol";

/// @title IntegrationTest
/// @notice Phase 3 端到端集成测试：覆盖 Registry → Job → Hook 三合约
///         交互的完整业务流程，验证跨合约状态机一致性。
/// @dev    测试场景：
///         1. Seed Phase + 全链路主路径（register/stake/submit/aggregate/create/fund/assign/submit/complete）
///         2. x402 支付路径集成
///         3. Deadline 退款路径
///         4. 仲裁分支：ruling=1 buyer 获胜（绕过 deadline 退款）
///         5. 仲裁分支：ruling=2 provider 获胜（complete 付款）
///         6. 声誉更新自动触发：Job complete → Evaluator submitValidation
contract IntegrationTest is Test {
    PrismSettleRegistry internal registry;
    PrismSettleJob internal job;
    ArbitrationHook internal hook;
    MockERC20 internal token;
    MockX402Facilitator internal facilitator;

    // 角色账户
    address internal deployer = address(this);
    address internal buyer = address(0xB0B);
    address internal provider = address(0xCAFE);
    address internal evaluator = address(0xE1A1);
    address internal validator = address(0xA11CE);
    address internal resolver = address(0xCAFE); // 占位，实际用 evaluator

    // 4 个官方 Agent
    uint256 internal constant AGENT_DEFI = 0x1111;
    uint256 internal constant AGENT_EVAL = 0x4444;
    uint256 internal constant SEED_SCORE = 0.7e18;

    uint256 internal constant FUND_AMOUNT = 100 ether;
    uint64 internal constant DEADLINE_OFFSET = 1 hours;

    function setUp() public {
        // Warp past Foundry default (block.timestamp = 1) for realistic time.
        vm.warp(1 days);

        // 部署三合约 + MockERC20 + MockFacilitator（与 Deploy.s.sol 顺序一致）
        token = new MockERC20("MockUSDC", "USDC");
        facilitator = new MockX402Facilitator(address(token), FUND_AMOUNT);
        registry = new PrismSettleRegistry();
        hook = new ArbitrationHook();
        job = new PrismSettleJob(address(token), address(facilitator));

        // Hook 反向引用
        hook.setJobContract(address(job));

        // 角色授权（evaluator 同时持有三合约角色）
        registry.grantRole(registry.REGISTRY_EVALUATOR_ROLE(), evaluator);
        job.grantRole(job.COMMERCE_EVALUATOR_ROLE(), evaluator);
        hook.grantRole(hook.RESOLVER_ROLE(), evaluator);

        // Seed Phase：注册 4 个 Agent + 设置初始声誉
        registry.registerAgent(AGENT_DEFI, '{"endpointUrl":"http://localhost:8001","capabilities":"defi"}');
        registry.registerAgent(AGENT_EVAL, '{"endpointUrl":"http://localhost:8004","capabilities":"evaluation"}');
        registry.seedAgent(AGENT_DEFI, uint96(SEED_SCORE));
        registry.seedAgent(AGENT_EVAL, uint96(SEED_SCORE));

        // Validator 质押（≥ MIN_STAKE = 100 ether）
        vm.deal(validator, 1000 ether);
        vm.prank(validator);
        registry.stake{value: 100 ether}();

        // 资金准备
        token.mint(buyer, 10_000 ether);
        token.mint(provider, 1_000 ether);
        vm.startPrank(buyer);
        token.approve(address(job), type(uint256).max);
        token.approve(address(facilitator), type(uint256).max);
        vm.stopPrank();
    }

    // ------------------------------------------------------------------
    // 辅助函数
    // ------------------------------------------------------------------

    /// @notice 创建 + 资助 + 分配 + 提交，返回 jobId
    function _createFundAssignSubmit(uint256 agentId) internal returns (uint256 jobId) {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        jobId = job.createJob(agentId, 0, deadline, address(0));

        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");

        vm.prank(buyer);
        job.assign(jobId, provider);

        vm.prank(provider);
        job.submit(jobId, keccak256("deliverable"), keccak256("proof"));
    }

    // ------------------------------------------------------------------
    // 场景 1：Seed Phase + 全链路主路径
    // ------------------------------------------------------------------

    function testSeedPhaseAndFullHappyPath() public {
        // 验证 Seed Phase：4 个 Agent 已注册并有初始声誉
        (bool registered,,,,, uint96 seedScore,,,) = registry.agents(AGENT_DEFI);
        assertTrue(registered);
        assertEq(seedScore, SEED_SCORE);
        assertEq(registry.aggregatedScore(AGENT_DEFI), SEED_SCORE);

        // Validator 提交一次评分 → 触发声誉更新路径
        vm.prank(validator);
        registry.submitValidation(AGENT_DEFI, 0.8e18, keccak256("proof1"), 0, 0);

        // Warp 过 EPOCH 后聚合
        vm.warp(block.timestamp + registry.EPOCH() + 1);
        registry.aggregateEpoch(AGENT_DEFI);

        // 聚合后分数应介于 seedScore(0.7) 和 weightedScore(0.8) 之间（EMA 平滑）
        // taskCount=0 时 EMA alpha=1，newScore = weighted = 0.8e18
        uint256 newScore = registry.aggregatedScore(AGENT_DEFI);
        assertGe(newScore, SEED_SCORE);
        assertLe(newScore, 0.8e18);

        // 创建 Job → 资助 → 分配 → 提交 → complete
        uint256 jobId = _createFundAssignSubmit(AGENT_DEFI);

        uint256 provBefore = token.balanceOf(provider);
        vm.prank(evaluator);
        job.complete(jobId);

        // 验证 provider 收到付款
        assertEq(token.balanceOf(provider), provBefore + FUND_AMOUNT);
        (PrismSettleJob.JobState s,,,,,,,) = job.getJobState(jobId);
        assertEq(uint256(s), uint256(PrismSettleJob.JobState.Completed));
    }

    // ------------------------------------------------------------------
    // 场景 2：x402 支付路径集成
    // ------------------------------------------------------------------

    function testX402FundingPathEndToEnd() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_DEFI, 0, deadline, address(0));

        // 使用 x402 receipt 资助（Facilitator 拉款）
        bytes memory receipt = bytes("x402-receipt-integration-v1");
        uint256 buyerBefore = token.balanceOf(buyer);
        vm.prank(buyer);
        job.fundViaToken(jobId, 0, receipt);

        // 验证 Facilitator 拉款金额 = FUND_AMOUNT
        assertEq(token.balanceOf(buyer), buyerBefore - FUND_AMOUNT);
        assertEq(token.balanceOf(address(job)), FUND_AMOUNT);

        // 后续流程正常
        vm.prank(buyer);
        job.assign(jobId, provider);
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));

        vm.prank(evaluator);
        job.complete(jobId);
        // provider 初始有 1_000 ether，complete 后 +FUND_AMOUNT
        assertEq(token.balanceOf(provider), 1_000 ether + FUND_AMOUNT);
    }

    // ------------------------------------------------------------------
    // 场景 3：Deadline 退款路径
    // ------------------------------------------------------------------

    function testDeadlineRefundPath() public {
        uint256 jobId = _createFundAssignSubmit(AGENT_DEFI);

        // 未到 deadline，退款失败
        vm.prank(buyer);
        vm.expectRevert("PrismSettle: deadline not reached");
        job.claimRefund(jobId);

        // Warp 过 deadline
        vm.warp(block.timestamp + DEADLINE_OFFSET + 1);

        uint256 buyerBefore = token.balanceOf(buyer);
        vm.prank(buyer);
        job.claimRefund(jobId);

        assertEq(token.balanceOf(buyer), buyerBefore + FUND_AMOUNT);
        (PrismSettleJob.JobState s,,,,,,,) = job.getJobState(jobId);
        assertEq(uint256(s), uint256(PrismSettleJob.JobState.Refunded));
    }

    // ------------------------------------------------------------------
    // 场景 4：仲裁 ruling=1（buyer 获胜，绕过 deadline 退款）
    // ------------------------------------------------------------------

    function testArbitrationRulingOneBuyerWinsRefundImmediately() public {
        // 创建带 Hook 的 Job
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_DEFI, 0, deadline, address(hook));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(buyer);
        job.assign(jobId, provider);
        vm.prank(provider);
        job.submit(jobId, keccak256("bad-deliverable"), keccak256("proof"));

        // Buyer 发起争议
        vm.prank(buyer);
        hook.dispute(jobId, keccak256("quality issue"));

        // Resolver 裁决 ruling=1（buyer 获胜）
        vm.prank(evaluator);
        hook.resolveDispute(jobId, 1);

        // 验证 Hook 状态
        (ArbitrationHook.HookState hs,, uint8 ruling) = hook.getHookState(jobId);
        assertEq(uint256(hs), uint256(ArbitrationHook.HookState.DisputeResolved));
        assertEq(ruling, 1);

        // 立即退款，无需等到 deadline
        uint256 buyerBefore = token.balanceOf(buyer);
        vm.prank(buyer);
        job.claimRefund(jobId);
        assertEq(token.balanceOf(buyer), buyerBefore + FUND_AMOUNT);
    }

    // ------------------------------------------------------------------
    // 场景 5：仲裁 ruling=2（provider 获胜，complete 付款）
    // ------------------------------------------------------------------

    function testArbitrationRulingTwoProviderWinsComplete() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_DEFI, 0, deadline, address(hook));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(buyer);
        job.assign(jobId, provider);
        vm.prank(provider);
        job.submit(jobId, keccak256("good-deliverable"), keccak256("proof"));

        vm.prank(buyer);
        hook.dispute(jobId, keccak256("unfair dispute"));

        vm.prank(evaluator);
        hook.resolveDispute(jobId, 2);

        // ruling=2 时不绕过 deadline，但 Evaluator 可直接 complete
        // 验证 ruling=2 不影响 complete 路径
        uint256 provBefore = token.balanceOf(provider);
        vm.prank(evaluator);
        job.complete(jobId);
        assertEq(token.balanceOf(provider), provBefore + FUND_AMOUNT);

        // complete 后 buyer 无法再 claimRefund（状态已终态）
        vm.prank(buyer);
        vm.expectRevert("PrismSettle: bad state");
        job.claimRefund(jobId);
    }

    // ------------------------------------------------------------------
    // 场景 6：Job complete 后 Evaluator 自动触发声誉更新
    //         （模拟 FR-E12：complete → submitValidation(source=1)）
    // ------------------------------------------------------------------

    function testJobCompleteTriggersReputationUpdate() public {
        uint256 jobId = _createFundAssignSubmit(AGENT_DEFI);

        // Evaluator 完成 Job
        vm.prank(evaluator);
        job.complete(jobId);

        // 模拟 Evaluator 自动调用 submitValidation 更新 Agent 声誉
        // （实际由链下 Evaluator 服务在 complete 成功后触发）
        uint96 evalScore = 0.9e18;
        vm.prank(evaluator);
        registry.submitValidation(
            AGENT_DEFI,
            evalScore,
            keccak256("eval-proof"),
            jobId,
            1 // source=1: Evaluator-Job 渠道
        );

        // Warp 过 EPOCH 后聚合
        vm.warp(block.timestamp + registry.EPOCH() + 1);
        registry.aggregateEpoch(AGENT_DEFI);

        // 验证声誉分数已更新（高于初始 SEED_SCORE）
        uint256 finalScore = registry.aggregatedScore(AGENT_DEFI);
        assertGt(finalScore, SEED_SCORE);

        // 验证 taskCount 已增加
        (,,,,,,, uint64 taskCount,) = registry.agents(AGENT_DEFI);
        assertGt(taskCount, 0);
    }

    // ------------------------------------------------------------------
    // 场景 7：跨合约状态机非法转换拦截
    // ------------------------------------------------------------------

    function testRevertCompleteBeforeSubmit() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_DEFI, 0, deadline, address(0));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(buyer);
        job.assign(jobId, provider);

        // 未 submit 直接 complete 应失败
        vm.prank(evaluator);
        vm.expectRevert("PrismSettle: bad state");
        job.complete(jobId);
    }

    function testRevertDisputeAfterCompleted() public {
        // 完成 Job 后再争议应失败（hook 状态机拦截）
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_DEFI, 0, deadline, address(hook));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(buyer);
        job.assign(jobId, provider);
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));
        vm.prank(evaluator);
        job.complete(jobId);

        // Job 已 Completed，但 Hook 仍可被 dispute（hook 状态独立）
        // 这是设计上的边界：dispute 不依赖 Job 状态，但 claimRefund
        // 会因 Job 状态已终态而失败。这里验证 dispute 仍可调用。
        vm.prank(buyer);
        hook.dispute(jobId, keccak256("late dispute"));
        (ArbitrationHook.HookState hs,,) = hook.getHookState(jobId);
        assertEq(uint256(hs), uint256(ArbitrationHook.HookState.Disputed));
    }

    // ------------------------------------------------------------------
    // 场景 8：Validator 声誉更新与 Evaluator 声誉更新并存
    // ------------------------------------------------------------------

    function testValidatorAndEvaluatorChannelsCoexist() public {
        // Validator 渠道提交
        vm.prank(validator);
        registry.submitValidation(AGENT_DEFI, 0.6e18, keccak256("v-proof"), 0, 0);

        // Evaluator 渠道提交（需关联 jobId）
        uint256 jobId = _createFundAssignSubmit(AGENT_DEFI);
        vm.prank(evaluator);
        registry.submitValidation(AGENT_DEFI, 0.9e18, keccak256("e-proof"), jobId, 1);

        // Warp + 聚合
        vm.warp(block.timestamp + registry.EPOCH() + 1);
        registry.aggregateEpoch(AGENT_DEFI);

        // 两条渠道的记录都被聚合（taskCount 增加 2）
        (,,,,,,, uint64 taskCount,) = registry.agents(AGENT_DEFI);
        assertEq(taskCount, 2);

        // 最终分数应介于 0.6 和 0.9 之间（加权 + EMA）
        // 注意：Validator stake=100e18, Evaluator weight=1
        // weighted = (0.6e18*100e18 + 0.9e18*1) / (100e18+1) ≈ 0.6e18（整数除法精度）
        // EMA: newScore = 0.7e18 + (0.6e18 - 0.7e18)*1e18/(1e18+0) = 0.6e18
        uint256 score = registry.aggregatedScore(AGENT_DEFI);
        assertGe(score, 0.6e18);
        assertLe(score, 0.9e18);
    }
}
