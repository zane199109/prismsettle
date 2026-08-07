// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Test} from "forge-std/Test.sol";
import {IAccessControl} from "@openzeppelin/contracts/access/IAccessControl.sol";
import {PrismSettleRegistry} from "../src/PrismSettleRegistry.sol";

/// @title PrismSettleRegistryTest
/// @notice Functional tests for the sharded validation registry.
///         Covers SD §3.2 design: 256-shard storage, EMA smoothing, inactivity
///         decay, dual-channel submitValidation, unstake lock-up, Seed Phase.
///         The sharded-vs-baseline abort-rate benchmark lives in a separate
///         load-test binary; this file covers correctness only.
contract PrismSettleRegistryTest is Test {
    PrismSettleRegistry internal reg;

    address internal validator = address(0xA11CE);
    address internal otherValidator = address(0xB0B);
    address internal agentOwner = address(0x0FF1CE);
    address internal evaluator = address(0xE1A1);

    uint256 internal constant AGENT_A = 0x1111; // shard 0x11
    uint256 internal constant AGENT_B = 0x2222; // shard 0x22
    uint256 internal constant AGENT_SAME_SHARD_A = 0x1313; // shard 0x13
    uint256 internal constant AGENT_SAME_SHARD_B = 0x4313; // shard 0x13

    string internal constant METADATA_A = "https://defi-agent.example/.well-known/agent.json";

    function setUp() public {
        reg = new PrismSettleRegistry();

        // Fund and stake the validators.
        vm.deal(validator, 1000 ether);
        vm.deal(otherValidator, 1000 ether);

        vm.prank(validator);
        reg.stake{value: 10 ether}();
        vm.prank(otherValidator);
        reg.stake{value: 10 ether}();

        // Grant Evaluator role.
        reg.grantRole(reg.REGISTRY_EVALUATOR_ROLE(), evaluator);

        // Foundry starts at block.timestamp = 1s. Warp forward so the first
        // aggregateEpoch call in any test is not blocked by EPOCH.
        vm.warp(2 minutes);
    }

    // ------------------------------------------------------------------
    // shardOf
    // ------------------------------------------------------------------

    function testShardOfReturnsLow8Bits() public view {
        assertEq(uint256(reg.shardOf(0xABCD)), 0xCD);
        assertEq(uint256(reg.shardOf(0x0100)), 0x00);
        assertEq(uint256(reg.shardOf(0x01FF)), 0xFF);
    }

    function testAgentsInSameShardShareShard() public view {
        assertEq(uint256(reg.shardOf(AGENT_SAME_SHARD_A)), uint256(reg.shardOf(AGENT_SAME_SHARD_B)));
    }

    // ------------------------------------------------------------------
    // registerAgent (FR-C01)
    // ------------------------------------------------------------------

    function testRegisterAgentSetsOwnerAndMetadata() public {
        vm.prank(agentOwner);
        vm.expectEmit(true, true, false, true);
        emit PrismSettleRegistry.AgentRegistered(AGENT_A, agentOwner, METADATA_A);
        reg.registerAgent(AGENT_A, METADATA_A);

        (
            bool registered,
            address owner,
            string memory endpointUrl,
            string memory capabilities,
            uint64 lastAggregate,
            uint96 seedScore,
            uint64 registeredAt,
            uint64 taskCount,
            uint64 lastActivity
        ) = reg.agents(AGENT_A);
        assertTrue(registered);
        assertEq(owner, agentOwner);
        assertEq(endpointUrl, METADATA_A);
        assertEq(capabilities, "");
        assertEq(uint256(lastAggregate), 0);
        assertEq(uint256(seedScore), 0);
        assertGt(uint256(registeredAt), 0);
        assertEq(uint256(taskCount), 0);
        assertEq(uint256(lastActivity), 0);
    }

    function testRevertDoubleRegister() public {
        vm.startPrank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);
        vm.expectRevert("PrismSettle: already registered");
        reg.registerAgent(AGENT_A, METADATA_A);
        vm.stopPrank();
    }

    // ------------------------------------------------------------------
    // seedAgent (FR-C09)
    // ------------------------------------------------------------------

    function testSeedAgentSetsSeedScoreAndAggregatedScore() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        reg.seedAgent(AGENT_A, 0.7e18);

        assertEq(reg.getScore(AGENT_A), 0.7e18);
        // seedScore field set (read via agents mapping tuple).
        (,,,,, uint96 seedScore,,,) = reg.agents(AGENT_A);
        assertEq(uint256(seedScore), 0.7e18);
    }

    function testRevertSeedAgentIfNotOwner() public {
        bytes32 adminRole = reg.DEFAULT_ADMIN_ROLE();

        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        vm.prank(agentOwner);
        vm.expectRevert(
            abi.encodeWithSelector(IAccessControl.AccessControlUnauthorizedAccount.selector, agentOwner, adminRole)
        );
        reg.seedAgent(AGENT_A, 0.7e18);
    }

    function testRevertSeedAgentIfAlreadySeeded() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);
        reg.seedAgent(AGENT_A, 0.7e18);

        vm.expectRevert("PrismSettle: already seeded");
        reg.seedAgent(AGENT_A, 0.8e18);
    }

    function testRevertSeedAgentIfNotRegistered() public {
        vm.expectRevert("PrismSettle: agent not registered");
        reg.seedAgent(AGENT_B, 0.7e18);
    }

    // ------------------------------------------------------------------
    // stake / unstake / withdrawUnstaked (FR-C02 / FR-C10)
    // ------------------------------------------------------------------

    function testStakeAccumulates() public {
        vm.prank(validator);
        reg.stake{value: 50 ether}();
        (uint256 amount,,) = reg.validatorStake(validator);
        assertEq(amount, 60 ether);
    }

    function testStakeZeroReverts() public {
        vm.prank(validator);
        vm.expectRevert("PrismSettle: zero stake");
        reg.stake{value: 0}();
    }

    function testUnstakeEntersLockPeriod() public {
        uint64 unlockAt = uint64(block.timestamp) + uint64(7 days);
        vm.prank(validator);
        vm.expectEmit(true, false, false, true);
        emit PrismSettleRegistry.UnstakeStarted(validator, 6 ether, unlockAt);
        reg.unstake(6 ether);

        (uint256 amount, uint256 pending, uint64 unstakeAt) = reg.validatorStake(validator);
        assertEq(amount, 4 ether, "remaining stake");
        assertEq(pending, 6 ether, "pending unstake");
        assertGt(uint256(unstakeAt), 0, "unstakeAt set");
    }

    function testRevertUnstakeInsufficientStake() public {
        vm.prank(validator);
        vm.expectRevert("PrismSettle: insufficient stake");
        reg.unstake(10_000 ether);
    }

    function testRevertUnstakeWhileAlreadyUnstaking() public {
        vm.prank(validator);
        reg.unstake(6 ether);

        vm.prank(validator);
        vm.expectRevert("PrismSettle: already unstaking");
        reg.unstake(1 ether);
    }

    function testRevertWithdrawBeforeLockExpires() public {
        vm.prank(validator);
        reg.unstake(6 ether);

        vm.prank(validator);
        vm.expectRevert("PrismSettle: lock not expired");
        reg.withdrawUnstaked();
    }

    function testWithdrawUnstakedSucceedsAfterLock() public {
        uint256 before = validator.balance;
        vm.prank(validator);
        reg.unstake(6 ether);

        // Warp past the 7-day lock.
        vm.warp(block.timestamp + 7 days + 1);

        vm.prank(validator);
        vm.expectEmit(true, false, false, true);
        emit PrismSettleRegistry.UnstakeWithdrawn(validator, 6 ether);
        reg.withdrawUnstaked();

        // Withdraw pays out pendingUnstake (6 ether); active stake (4 ether) untouched.
        assertEq(validator.balance, before + 6 ether);
        (uint256 afterAmount, uint256 afterPending, uint64 afterUnstakeAt) = reg.validatorStake(validator);
        assertEq(afterAmount, 4 ether, "active stake preserved");
        assertEq(afterPending, 0, "pending cleared");
        assertEq(uint256(afterUnstakeAt), 0);
    }

    // ------------------------------------------------------------------
    // submitValidation (FR-C03)
    // ------------------------------------------------------------------

    function testSubmitValidationValidatorChannelAppendsToShard() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.5e18, keccak256("proof"), 0, 0);

        assertEq(reg.getValidationCount(AGENT_A), 1);
        (address v, uint96 score, bytes32 proof, uint64 ts, uint8 source, uint256 jobId) = reg.getValidation(AGENT_A, 0);
        assertEq(v, validator);
        assertEq(uint256(score), 0.5e18);
        assertEq(proof, keccak256("proof"));
        assertGt(uint256(ts), 0);
        assertEq(uint256(source), 0);
        assertEq(jobId, 0);
    }

    function testSubmitValidationEvaluatorChannel() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        vm.prank(evaluator);
        reg.submitValidation(AGENT_A, 0.8e18, keccak256("proof"), 1234, 1);

        assertEq(reg.getValidationCount(AGENT_A), 1);
        (,,,, uint8 source, uint256 jobId) = reg.getValidation(AGENT_A, 0);
        assertEq(uint256(source), 1);
        assertEq(jobId, 1234);
    }

    function testSubmitValidationUpdatesLastActivity() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        uint256 ts = block.timestamp;
        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.5e18, keccak256("proof"), 0, 0);

        (,,,,,,,, uint64 lastActivity) = reg.agents(AGENT_A);
        assertEq(uint256(lastActivity), ts);
    }

    function testRevertSubmitToUnregisteredAgent() public {
        vm.prank(validator);
        vm.expectRevert("PrismSettle: agent not registered");
        reg.submitValidation(AGENT_A, 0.5e18, keccak256("proof"), 0, 0);
    }

    function testRevertSubmitWithInsufficientStake() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        address poor = address(0xC0FFEE);
        vm.deal(poor, 10 ether);
        vm.startPrank(poor);
        reg.stake{value: 3 ether}();
        vm.expectRevert("PrismSettle: insufficient stake");
        reg.submitValidation(AGENT_A, 0.5e18, keccak256("proof"), 0, 0);
        vm.stopPrank();
    }

    function testRevertScoreAboveMax() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        vm.prank(validator);
        vm.expectRevert("PrismSettle: score > 1e18");
        reg.submitValidation(AGENT_A, 1e18 + 1, keccak256("proof"), 0, 0);
    }

    function testRevertValidatorJobIdNotZero() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        vm.prank(validator);
        vm.expectRevert("PrismSettle: validator jobId must be 0");
        reg.submitValidation(AGENT_A, 0.5e18, keccak256("proof"), 1, 0);
    }

    function testRevertEvaluatorWithoutRole() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        address fakeEvaluator = address(0xFA1);
        vm.prank(fakeEvaluator);
        vm.expectRevert("PrismSettle: not evaluator");
        reg.submitValidation(AGENT_A, 0.5e18, keccak256("proof"), 1, 1);
    }

    function testRevertInvalidSource() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        vm.prank(validator);
        vm.expectRevert("PrismSettle: invalid source");
        reg.submitValidation(AGENT_A, 0.5e18, keccak256("proof"), 0, 4);
    }

    function testSubmitValidationEnforcesEpochQuota() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        // Submit MAX_VALIDATIONS_PER_EPOCH records (50).
        for (uint256 i = 0; i < reg.MAX_VALIDATIONS_PER_EPOCH(); i++) {
            vm.prank(validator);
            reg.submitValidation(AGENT_A, 0.5e18, keccak256(abi.encode(i)), 0, 0);
        }

        // 51st should revert.
        vm.prank(validator);
        vm.expectRevert("PrismSettle: validator epoch quota exceeded");
        reg.submitValidation(AGENT_A, 0.5e18, keccak256("overflow"), 0, 0);
    }

    function testSubmitValidationEvaluatorExemptFromQuota() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        // Evaluator records are capped by MAX_EVALUATOR_RECORDS_PER_EPOCH (20),
        // independent of the Validator quota. 21st reverts.
        for (uint256 i = 0; i < reg.MAX_EVALUATOR_RECORDS_PER_EPOCH(); i++) {
            vm.prank(evaluator);
            reg.submitValidation(AGENT_A, 0.5e18, keccak256(abi.encode(i)), i, 1);
        }

        assertEq(reg.getValidationCount(AGENT_A), reg.MAX_EVALUATOR_RECORDS_PER_EPOCH());

        vm.prank(evaluator);
        vm.expectRevert("PrismSettle: evaluator epoch quota exceeded");
        reg.submitValidation(AGENT_A, 0.5e18, keccak256("overflow"), 21, 1);
    }

    function testMultipleValidatorsAppendIndependently() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.4e18, bytes32(uint256(1)), 0, 0);
        vm.prank(otherValidator);
        reg.submitValidation(AGENT_A, 0.6e18, bytes32(uint256(2)), 0, 0);

        assertEq(reg.getValidationCount(AGENT_A), 2);
    }

    // ------------------------------------------------------------------
    // aggregateEpoch (FR-C04) — stake-weighted + EMA + decay
    // ------------------------------------------------------------------

    function testAggregateComputesStakeWeightedScore() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        // validator (100 stake) gives 0.4, otherValidator (100 stake) gives 0.6
        // Fixed ALPHA=0.3: newScore = 0 + 0.3 * (0.5e18 - 0) = 0.15e18
        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.4e18, bytes32(uint256(1)), 0, 0);
        vm.prank(otherValidator);
        reg.submitValidation(AGENT_A, 0.6e18, bytes32(uint256(2)), 0, 0);

        reg.aggregateEpoch(AGENT_A);

        assertEq(reg.getScore(AGENT_A), 0.15e18);
    }

    function testAggregateEmitsAggregatedEvent() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);
        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.7e18, bytes32(uint256(1)), 0, 0);

        // Aggregated(agentId, oldScore, newScore, count, taskCount, decay)
        vm.expectEmit(true, false, false, true);
        emit PrismSettleRegistry.Aggregated(AGENT_A, 0, 0.21e18, 1, 1, 0);
        reg.aggregateEpoch(AGENT_A);
    }

    function testRevertAggregateBeforeEpoch() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);
        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.5e18, bytes32(uint256(1)), 0, 0);

        reg.aggregateEpoch(AGENT_A);

        vm.expectRevert("PrismSettle: epoch not due");
        reg.aggregateEpoch(AGENT_A);
    }

    function testAggregateOnEmptyShardAdvancesTimestamp() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        // Should not revert and should not change score.
        reg.aggregateEpoch(AGENT_A);
        assertEq(reg.getScore(AGENT_A), 0);
    }

    function testAggregateUnregisteredAgentReverts() public {
        vm.expectRevert("PrismSettle: agent not registered");
        reg.aggregateEpoch(AGENT_B);
    }

    function testAggregateEMASmoothingOnSecondEpoch() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        // First epoch: 0.5 → 0 + 0.3 × 0.5 = 0.15e18 (fixed ALPHA)
        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.5e18, bytes32(uint256(1)), 0, 0);
        reg.aggregateEpoch(AGENT_A);
        assertEq(reg.getScore(AGENT_A), 0.15e18);

        // Second epoch: 1.0 → 0.15 + 0.3 × (1.0 - 0.15) = 0.405e18
        vm.warp(block.timestamp + 2 minutes);
        vm.prank(otherValidator);
        reg.submitValidation(AGENT_A, 1.0e18, bytes32(uint256(2)), 0, 0);
        reg.aggregateEpoch(AGENT_A);

        uint256 score = reg.getScore(AGENT_A);
        assertApproxEqAbs(score, 0.405e18, 1);
    }

    function testAggregateClearsProcessedRecords() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.5e18, bytes32(uint256(1)), 0, 0);
        reg.aggregateEpoch(AGENT_A);

        // Records should be cleared after aggregation.
        assertEq(reg.getValidationCount(AGENT_A), 0);
    }

    function testAggregateTruncatesAtMaxRecords() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        // Submit MAX_RECORDS + 10 records via the arbitration channel
        // (source=2, exempt from per-epoch quotas) so aggregateEpoch's
        // MAX_RECORDS truncation can be verified.
        for (uint256 i = 0; i < reg.MAX_RECORDS() + 10; i++) {
            vm.prank(evaluator);
            reg.submitValidation(AGENT_A, 0.5e18, keccak256(abi.encode(i)), uint64(i), 2);
        }

        reg.aggregateEpoch(AGENT_A);

        // MAX_RECORDS processed, 10 remain.
        assertEq(reg.getValidationCount(AGENT_A), 10);
    }

    // ------------------------------------------------------------------
    // getScore + inactivity decay (SD §3.2.5 applyDecay)
    // ------------------------------------------------------------------

    function testGetScoreReturnsStoredScoreWhenActive() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);
        reg.seedAgent(AGENT_A, 0.7e18);

        // lastActivity = 0 (no submitValidation yet) => applyDecay returns score unchanged.
        assertEq(reg.getScore(AGENT_A), 0.7e18);
    }

    function testGetScoreAppliesInactivityDecay() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);
        reg.seedAgent(AGENT_A, 0.9e18);

        // Submit a validation to set lastActivity, then warp 40 days.
        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.9e18, keccak256("proof"), 0, 0);
        uint256 lastActivity = block.timestamp;

        // Warp 40 days forward: 10 days past INACTIVE_DAYS (30).
        vm.warp(lastActivity + 40 days);

        // decay = 10 * 0.01e18 = 0.1e18
        // But stored aggregatedScore is still 0.9e18 (no aggregateEpoch called).
        // getScore applies decay at read time: 0.9e18 - 0.1e18 = 0.8e18.
        assertEq(reg.getScore(AGENT_A), 0.8e18);
    }

    function testGetScoreDecayFloorsAtZero() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);
        reg.seedAgent(AGENT_A, 0.05e18);

        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.05e18, keccak256("proof"), 0, 0);
        uint256 lastActivity = block.timestamp;

        // Warp 100 days forward: 70 days past INACTIVE_DAYS.
        // decay = 70 * 0.01e18 = 0.7e18 > 0.05e18 => floor at 0.
        vm.warp(lastActivity + 100 days);

        assertEq(reg.getScore(AGENT_A), 0);
    }

    // ------------------------------------------------------------------
    // slashing (FR-C05, V1: Owner only)
    // ------------------------------------------------------------------

    function testSlashZeroBalanceReverts() public {
        address empty = address(0xDEAD);
        vm.expectRevert("PrismSettle: nothing to slash");
        reg.slash(empty, keccak256("no coord"));
    }

    function testRevertSlashIfNotOwner() public {
        bytes32 adminRole = reg.DEFAULT_ADMIN_ROLE();

        vm.prank(agentOwner);
        vm.expectRevert(
            abi.encodeWithSelector(IAccessControl.AccessControlUnauthorizedAccount.selector, agentOwner, adminRole)
        );
        reg.slash(validator, keccak256("sybil"));
    }

    function testSlashBurnsStakeAndZeroesBalance() public {
        uint256 before = validator.balance;
        uint256 deadBefore = address(0xdead).balance;

        reg.slash(validator, keccak256("sybil"));

        (uint256 amount,,) = reg.validatorStake(validator);
        assertEq(amount, 0);
        assertEq(validator.balance, before);
        assertEq(address(0xdead).balance, deadBefore + 10 ether);
    }

    function testSlashedValidatorCannotSubmit() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);
        reg.slash(validator, keccak256("sybil"));

        vm.prank(validator);
        vm.expectRevert("PrismSettle: insufficient stake");
        reg.submitValidation(AGENT_A, 0.5e18, bytes32(uint256(1)), 0, 0);
    }

    function testSlashedValidatorWeightedAsOneInAggregate() public {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);

        // 10-stake validator gives 0.4, then gets slashed.
        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.4e18, bytes32(uint256(1)), 0, 0);
        reg.slash(validator, keccak256("sybil"));

        // 10-stake validator gives 0.6.
        vm.prank(otherValidator);
        reg.submitValidation(AGENT_A, 0.6e18, bytes32(uint256(2)), 0, 0);

        // After slash, validator weight falls back to 1. The other
        // validator's stake is 10 ether, so the slashed record's
        // contribution is effectively negligible.
        // First aggregate: oldScore=0, taskCount=0, alpha=1 => newScore = weighted.
        uint256 w0 = 1;
        uint256 w1 = 10 ether;
        uint256 expected = (uint256(0.4e18) * w0 + uint256(0.6e18) * w1) / (w0 + w1);
        reg.aggregateEpoch(AGENT_A);
        // Fixed ALPHA=0.3: newScore = 0 + 0.3 × weighted.
        assertEq(reg.getScore(AGENT_A), expected * reg.ALPHA() / 1e18);
    }

    // ------------------------------------------------------------------
    // claimRewards (FR-C11, V1 stub)
    // ------------------------------------------------------------------

    function testClaimRewardsReverts() public {
        vm.expectRevert("PrismSettle: V2 only");
        reg.claimRewards();
    }

    // ------------------------------------------------------------------
    // 256-shard isolation: agents in different shards don't collide
    // ------------------------------------------------------------------

    function testShardIsolationDifferentShards() public {
        // AGENT_A (shard 0x11) and AGENT_B (shard 0x22) should have
        // independent validation arrays.
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_B, "https://other.example/agent.json");

        vm.prank(validator);
        reg.submitValidation(AGENT_A, 0.5e18, bytes32(uint256(1)), 0, 0);
        vm.prank(validator);
        reg.submitValidation(AGENT_B, 0.6e18, bytes32(uint256(2)), 0, 0);

        assertEq(reg.getValidationCount(AGENT_A), 1);
        assertEq(reg.getValidationCount(AGENT_B), 1);
        assertEq(uint256(reg.shardOf(AGENT_A)), 0x11);
        assertEq(uint256(reg.shardOf(AGENT_B)), 0x22);
    }

    // ------------------------------------------------------------------
    // Dual-factor aggregation — Buyer ratings (source=3)
    // ------------------------------------------------------------------

    function _buyerRatingSetup() internal {
        vm.prank(agentOwner);
        reg.registerAgent(AGENT_A, METADATA_A);
        reg.seedAgent(AGENT_A, 0.7e18);
        reg.setTrustedJob(address(this), true);
        vm.warp(block.timestamp + reg.EPOCH() + 1);
    }

    /// @notice Single 0.9 rating: completion bonus (+0.005) + rating nudge
    ///         ((0.9-0.5)*0.05 = +0.02) → 0.7 + 0.005 + 0.02 = 0.725.
    function testBuyerRatingDualFactorAdjustment() public {
        _buyerRatingSetup();
        reg.submitValidation(AGENT_A, 0.9e18, keccak256("p"), 1, 3);
        reg.aggregateEpoch(AGENT_A);
        assertEq(reg.getScore(AGENT_A), 0.725e18, "bonus + positive nudge");
    }

    /// @notice Low rating 0.1: bonus +0.005, nudge (0.1-0.5)*0.05 = -0.02
    ///         → 0.7 + 0.005 - 0.02 = 0.685.
    function testBuyerRatingDownwardAdjustment() public {
        _buyerRatingSetup();
        reg.submitValidation(AGENT_A, 0.1e18, keccak256("p"), 1, 3);
        reg.aggregateEpoch(AGENT_A);
        assertEq(reg.getScore(AGENT_A), 0.685e18, "bonus + negative nudge");
    }

    /// @notice 11 completions in one epoch: bonus capped at 0.05 and nudge
    ///         capped at 0.02 → 0.7 + 0.05 + 0.02 = 0.77 (anti-sybil).
    function testCompletionBonusEpochCap() public {
        _buyerRatingSetup();
        for (uint256 i = 0; i < 11; i++) {
            reg.submitValidation(AGENT_A, 0.9e18, keccak256(abi.encode(i)), i + 1, 3);
        }
        reg.aggregateEpoch(AGENT_A);
        assertEq(reg.getScore(AGENT_A), 0.77e18, "capped bonus + capped nudge");
    }

    /// @notice source=3 only (no weighted records) → no EMA pull: a neutral
    ///         0.5 rating leaves only the completion bonus → 0.705.
    function testBuyerRatingNoEmaWithoutWeightedRecords() public {
        _buyerRatingSetup();
        reg.submitValidation(AGENT_A, 0.5e18, keccak256("p"), 1, 3);
        reg.aggregateEpoch(AGENT_A);
        assertEq(reg.getScore(AGENT_A), 0.705e18, "neutral rating keeps bonus only");
    }

}
