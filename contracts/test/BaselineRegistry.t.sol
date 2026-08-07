// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Test} from "forge-std/Test.sol";
import {BaselineRegistry} from "../src/bench/BaselineRegistry.sol";

/// @title BaselineRegistryTest
/// @notice V0 对照合约 smoke 测试：验证 V0 与 V1 行为一致（仅存储路径不同）。
///         完整功能测试由 PrismSettleRegistry.t.sol 覆盖，这里仅做关键路径回归。
contract BaselineRegistryTest is Test {
    BaselineRegistry internal reg;
    address internal validator = address(0xA11CE);
    uint256 internal constant AGENT_A = 0x1111;

    function setUp() public {
        reg = new BaselineRegistry();

        // Validator 质押
        vm.deal(validator, 1000 ether);
        vm.prank(validator);
        reg.stake{value: 100 ether}();

        // 注册 + seed Agent
        reg.registerAgent(AGENT_A, '{"endpointUrl":"http://localhost:8001"}');
        reg.seedAgent(AGENT_A, 0.7e18);

        // Warp 过 EPOCH 边界
        vm.warp(2 minutes);
    }

    function testSubmitValidationAppendsToSingleSlot() public {
        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.8e18, keccak256("proof"), 0, 0);

        assertEq(reg.getValidationCount(AGENT_A), 1);
        (address v, uint96 score,,, uint8 source,) = reg.getValidation(AGENT_A, 0);
        assertEq(v, validator);
        assertEq(score, 0.8e18);
        assertEq(source, 0);
    }

    function testAggregateEpochUpdatesScore() public {
        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.8e18, keccak256("proof"), 0, 0);

        vm.warp(block.timestamp + reg.EPOCH() + 1);
        reg.aggregateEpoch(AGENT_A);

        // taskCount=0 时 EMA alpha=1，newScore = weighted = 0.8e18
        assertEq(reg.aggregatedScore(AGENT_A), 0.8e18);
    }

    function testV0HasNoShardIsolation() public {
        // V0 核心特征：所有 agent 的记录写入同一 mapping（单槽）。
        // 验证 V0 的 validations[agentId] 直接可访问，无 shard 中间层。
        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.8e18, keccak256("proof"), 0, 0);

        // 直接读 validations[AGENT_A] —— V0 的存储路径
        (address v,,,,,) = reg.getValidation(AGENT_A, 0);
        assertEq(v, validator);

        // V0 的 ValidationSubmitted 事件 shard 字段恒为 0
        // （V1 中 shard = agentId & 0xFF，V0 中无分片概念）
    }

    function testSlashBurnsStake() public {
        uint256 deadBefore = address(0xdead).balance;
        reg.slash(validator, keccak256("sybil"));
        assertEq(address(0xdead).balance, deadBefore + 100 ether);
        // Stake 清零
        (uint256 amount,,) = reg.validatorStake(validator);
        assertEq(amount, 0);
    }
}
