// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Test} from "forge-std/Test.sol";
import {IAccessControl} from "@openzeppelin/contracts/access/IAccessControl.sol";
import {PrismSettleJob} from "../src/PrismSettleJob.sol";
import {ArbitrationHook} from "../src/ArbitrationHook.sol";
import {IX402Facilitator} from "../src/interfaces/IX402Facilitator.sol";
import {MockERC20} from "../src/mocks/MockERC20.sol";
import {MockX402Facilitator} from "../src/mocks/MockX402Facilitator.sol";
import {PrismSettleRegistry} from "../src/PrismSettleRegistry.sol";

/// @title PrismSettleJobTest
/// @notice Covers SD §3.3: create → fund (ERC-20 + x402) → assign →
///         submit → complete / claimRefund, including the arbitration
///         ruling=1 refund bypass and 256-shard isolation.
contract PrismSettleJobTest is Test {
    MockERC20 internal token;
    MockX402Facilitator internal facilitator;
    ArbitrationHook internal hook;
    PrismSettleJob internal job;
    PrismSettleRegistry internal registry;

    address internal buyer = address(0xB0B);
    address internal provider = address(0xCAFE);
    address internal evaluator = address(0xE1A1);
    address internal other = address(0xA11CE);

    uint256 internal constant AGENT_ID = 0x1111;
    uint256 internal constant AGENT_PROVIDER = 0x5555;
    uint256 internal constant EVALUATOR_AGENT_ID = 0x4444;
    uint256 internal constant FUND_AMOUNT = 100 ether; // 100 USDC (18 dec for mock)
    uint64 internal constant DEADLINE_OFFSET = 1 hours;
    uint256 internal constant DEPOSIT = 5 ether; // FUND_AMOUNT × DEPOSIT_BPS(500) / 10000

    function setUp() public {
        // Warp past the Foundry default (block.timestamp = 1) so deadline
        // arithmetic is realistic.
        vm.warp(1 days);

        token = new MockERC20("MockUSDC", "USDC");
        facilitator = new MockX402Facilitator(address(token), FUND_AMOUNT);
        hook = new ArbitrationHook(address(token));
        job = new PrismSettleJob(address(token), address(facilitator));

        // Deploy a real registry and register the provider agent.
        registry = new PrismSettleRegistry();
        // AGENT_PROVIDER is owned by the provider wallet (grabJob requires
        // caller == agent owner or delegated operator).
        vm.prank(provider);
        registry.registerAgent(AGENT_PROVIDER, '{"endpointUrl":"","capabilities":"provider"}');
        registry.seedAgent(AGENT_PROVIDER, uint96(0.7e18));

        // Register and seed the evaluator's agentId for reputation-based selection.
        registry.registerAgent(EVALUATOR_AGENT_ID, '{"endpointUrl":"","capabilities":"evaluator"}');
        registry.seedAgent(EVALUATOR_AGENT_ID, uint96(0.7e18));

        // Wire the registry into the Job contract for grabJob reputation checks.
        job.setRegistry(address(registry));

        // Trust the Job contract to call submitValidation(source=3) for Buyer ratings.
        registry.setTrustedJob(address(job), true);

        // Wire the Hook back-references — use the real registry.
        hook.setRegistryContract(address(registry));
        hook.setJobContract(address(job));

        // Register the evaluator as an arbitrator with default 5% fee.
        vm.prank(evaluator);
        hook.registerArbitrator(EVALUATOR_AGENT_ID, 500, address(0xFEE));

        // Grant Evaluator roles.
        hook.grantRole(hook.RESOLVER_ROLE(), evaluator);

        // Fund buyer and provider with USDC; buyer approves the contracts.
        token.mint(buyer, 10_000 ether);
        token.mint(provider, 10_000 ether);

        vm.startPrank(buyer);
        token.approve(address(job), type(uint256).max);
        // For x402 path, the Facilitator pulls from buyer.
        token.approve(address(facilitator), type(uint256).max);
        // Deposits (reject / dispute) are pulled by the Hook.
        token.approve(address(hook), type(uint256).max);
        vm.stopPrank();
        vm.prank(provider);
        token.approve(address(hook), type(uint256).max);
    }

    // ------------------------------------------------------------------
    // Helpers
    // ------------------------------------------------------------------

    /// @notice Create + fund (ERC-20) a Job, returning the jobId.
    function _createAndFund() internal returns (uint256 jobId) {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        jobId = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));

        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
    }

    /// @notice Helper: create a job with a hook that has an arbitrator with
    ///         the given fee configured. Returns the jobId after fund +
    ///         grabJob + submit.
    function _createFundAssignSubmitWithHook(uint256 feeBps, address feeRecipient)
        internal
        returns (uint256 jobId)
    {
        // Re-register the evaluator as an arbitrator with the desired fee.
        // Unregister first (in case the evaluator is already registered).
        vm.prank(evaluator);
        hook.unregisterArbitrator();
        vm.prank(evaluator);
        hook.registerArbitrator(EVALUATOR_AGENT_ID, feeBps, feeRecipient);

        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        jobId = job.createJob(AGENT_ID, 0, deadline, address(hook), 0, address(0));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(provider);
        job.grabJob(jobId, AGENT_PROVIDER);
        vm.prank(provider);
        job.submit(jobId, keccak256("deliverable"), keccak256("proof"));
    }


    /// @notice Create + fund + assign, returning the jobId.
    function _createFundAndAssign() internal returns (uint256 jobId) {
        jobId = _createAndFund();
        vm.prank(provider);
        job.grabJob(jobId, AGENT_PROVIDER);
    }

    // ------------------------------------------------------------------
    // createJob
    // ------------------------------------------------------------------

    function testCreateJobEmitsEvent() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        // jobId is derived from keccak256(buyer, nonce) and cannot be
        // predicted in expectEmit's indexed topic; we verify state instead.
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));

        (PrismSettleJob.JobState s, address jbBuyer, , , , , uint64 jbDeadline, address jbHook, , ) = job.getJobState(jobId);
        assertEq(uint256(s), uint256(PrismSettleJob.JobState.Created));
        assertEq(jbBuyer, buyer);
        assertEq(jbDeadline, deadline);
        assertEq(jbHook, address(0));
    }

    function testCreateJobIncrementsNonce() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.startPrank(buyer);
        uint256 id1 = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        uint256 id2 = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        vm.stopPrank();

        // Same buyer + different nonce → different jobId.
        assertNotEq(id1, id2);
        // buyerNonce incremented.
        assertEq(job.buyerNonce(buyer), 2);
        // Both land in potentially different shards (deterministic by hash).
        uint8 shard1 = uint8(id1 & 0xFF);
        uint8 shard2 = uint8(id2 & 0xFF);
        // We don't assert shards differ (hash-dependent), only that both exist.
        assertNotEq(shard1, shard1 + 1); // sanity: shard is a byte
    }

    function testRevertCreateJobDeadlinePassed() public {
        uint64 deadline = uint64(block.timestamp) - 1;
        vm.prank(buyer);
        vm.expectRevert("PrismSettle: deadline passed");
        job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
    }

    // ------------------------------------------------------------------
    // fundViaToken — ERC-20 path
    // ------------------------------------------------------------------

    function testFundViaTokenERC20Path() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));

        uint256 balBefore = token.balanceOf(buyer);
        vm.prank(buyer);
        vm.expectEmit(true, false, false, true);
        emit PrismSettleJob.Funded(jobId, buyer, FUND_AMOUNT);
        job.fundViaToken(jobId, FUND_AMOUNT, "");

        assertEq(token.balanceOf(buyer), balBefore - FUND_AMOUNT);
        assertEq(token.balanceOf(address(job)), FUND_AMOUNT);
        (PrismSettleJob.JobState s, , , uint256 amount, , , , , , ) = job.getJobState(jobId);
        assertEq(uint256(s), uint256(PrismSettleJob.JobState.Funded));
        assertEq(amount, FUND_AMOUNT);
    }

    function testRevertFundNotBuyer() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));

        vm.prank(other);
        vm.expectRevert("PrismSettle: not buyer");
        job.fundViaToken(jobId, FUND_AMOUNT, "");
    }

    function testRevertFundBadState() public {
        uint256 jobId = _createAndFund(); // already Funded

        vm.prank(buyer);
        vm.expectRevert("PrismSettle: bad state");
        job.fundViaToken(jobId, FUND_AMOUNT, "");
    }

    function testRevertFundERC20ZeroAmount() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));

        vm.prank(buyer);
        vm.expectRevert("PrismSettle: zero amount");
        job.fundViaToken(jobId, 0, "");
    }

    // ------------------------------------------------------------------
    // fundViaToken — x402 path
    // ------------------------------------------------------------------

    function testFundViaTokenX402Path() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));

        bytes memory receipt = bytes("x402-receipt-v1");
        uint256 balBefore = token.balanceOf(buyer);
        vm.prank(buyer);
        job.fundViaToken(jobId, 0, receipt);

        // Facilitator pulled FUND_AMOUNT from buyer → contract.
        assertEq(token.balanceOf(buyer), balBefore - FUND_AMOUNT);
        assertEq(token.balanceOf(address(job)), FUND_AMOUNT);
        (, , , uint256 amount, , , , , , ) = job.getJobState(jobId);
        assertEq(amount, FUND_AMOUNT);
    }

    function testRevertFundX402ReceiptReplay() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));

        bytes memory receipt = bytes("x402-receipt-v1");
        vm.prank(buyer);
        job.fundViaToken(jobId, 0, receipt);

        // Replay on a second Job.
        vm.prank(buyer);
        uint256 jobId2 = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        vm.prank(buyer);
        vm.expectRevert("PrismSettle: receipt used");
        job.fundViaToken(jobId2, 0, receipt);
    }

    // ------------------------------------------------------------------
    // assign
    // ------------------------------------------------------------------

    function testAssignTransitionsToAssigned() public {
        uint256 jobId = _createAndFund();

        vm.prank(provider);
        vm.expectEmit(true, false, false, true);
        emit PrismSettleJob.Assigned(jobId, provider);
        job.grabJob(jobId, AGENT_PROVIDER);

        (PrismSettleJob.JobState s, , address p, , , , , , , ) = job.getJobState(jobId);
        assertEq(uint256(s), uint256(PrismSettleJob.JobState.Assigned));
        assertEq(p, provider);
    }



    function testRevertAssignBadState() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        // Still Created, not Funded.
        vm.prank(buyer);
        vm.expectRevert("PrismSettle: bad state");
        job.grabJob(jobId, AGENT_PROVIDER);
    }



    // ------------------------------------------------------------------
    // submit
    // ------------------------------------------------------------------

    function testSubmitTransitionsToSubmitted() public {
        uint256 jobId = _createFundAndAssign();

        bytes32 deliv = keccak256("deliverable");
        bytes32 proof = keccak256("proof");
        vm.prank(provider);
        vm.expectEmit(true, false, false, true);
        emit PrismSettleJob.Submitted(jobId, deliv, proof);
        job.submit(jobId, deliv, proof);

        (PrismSettleJob.JobState s, , , , bytes32 d, bytes32 p, , , , ) = job.getJobState(jobId);
        assertEq(uint256(s), uint256(PrismSettleJob.JobState.Submitted));
        assertEq(d, deliv);
        assertEq(p, proof);
    }

    function testRevertSubmitNotProvider() public {
        uint256 jobId = _createFundAndAssign();

        vm.prank(other);
        vm.expectRevert("PrismSettle: not provider");
        job.submit(jobId, keccak256("d"), keccak256("p"));
    }

    function testRevertSubmitEmptyDeliverable() public {
        uint256 jobId = _createFundAndAssign();

        vm.prank(provider);
        vm.expectRevert("PrismSettle: empty deliverable");
        job.submit(jobId, bytes32(0), keccak256("p"));
    }

    function testRevertSubmitBadState() public {
        uint256 jobId = _createAndFund(); // Funded, not Assigned

        vm.prank(provider);
        vm.expectRevert("PrismSettle: bad state");
        job.submit(jobId, keccak256("d"), keccak256("p"));
    }

    // ------------------------------------------------------------------
    // complete — Evaluator only
    // ------------------------------------------------------------------

    function testCompletePaysProvider() public {
        uint256 jobId = _createFundAndAssign();
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));

        uint256 provBefore = token.balanceOf(provider);
        vm.prank(buyer);
        job.complete(jobId, uint96(0.9e18));

        assertEq(token.balanceOf(provider), provBefore + FUND_AMOUNT);
        (PrismSettleJob.JobState s, , , , , , , , , ) = job.getJobState(jobId);
        assertEq(uint256(s), uint256(PrismSettleJob.JobState.Completed));
    }

    function testRevertCompleteNotEvaluator() public {
        uint256 jobId = _createFundAndAssign();
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));

        vm.prank(other);
        vm.expectRevert("PrismSettle: not buyer");
        job.complete(jobId, uint96(0.9e18));
    }

    function testRevertCompleteBadState() public {
        uint256 jobId = _createFundAndAssign(); // Assigned, not Submitted

        vm.prank(evaluator);
        vm.expectRevert("PrismSettle: bad state");
        job.complete(jobId, uint96(0.9e18));
    }

    // ------------------------------------------------------------------
    // claimRefund
    // ------------------------------------------------------------------

    function testClaimRefundAfterDeadline() public {
        uint256 jobId = _createAndFund(); // Funded

        // Warp past deadline.
        vm.warp(block.timestamp + DEADLINE_OFFSET + 1);

        uint256 buyerBefore = token.balanceOf(buyer);
        vm.prank(buyer);
        vm.expectEmit(true, false, false, true);
        emit PrismSettleJob.Refunded(jobId, buyer, FUND_AMOUNT);
        job.claimRefund(jobId);

        assertEq(token.balanceOf(buyer), buyerBefore + FUND_AMOUNT);
        (PrismSettleJob.JobState s, , , , , , , , , ) = job.getJobState(jobId);
        assertEq(uint256(s), uint256(PrismSettleJob.JobState.Refunded));
    }

    function testRevertClaimRefundBeforeDeadline() public {
        uint256 jobId = _createAndFund();

        vm.prank(buyer);
        vm.expectRevert("PrismSettle: deadline not reached");
        job.claimRefund(jobId);
    }

    function testRevertClaimRefundNotBuyer() public {
        uint256 jobId = _createAndFund();
        vm.warp(block.timestamp + DEADLINE_OFFSET + 1);

        vm.prank(other);
        vm.expectRevert("PrismSettle: not buyer");
        job.claimRefund(jobId);
    }

    function testClaimRefundFromSubmittedState() public {
        uint256 jobId = _createFundAndAssign();
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));

        vm.warp(block.timestamp + DEADLINE_OFFSET + 1);
        uint256 buyerBefore = token.balanceOf(buyer);
        vm.prank(buyer);
        job.claimRefund(jobId);

        assertEq(token.balanceOf(buyer), buyerBefore + FUND_AMOUNT);
    }

    // ------------------------------------------------------------------
    // Arbitration ruling=1 bypasses deadline (FR-J08 / SD §3.3.5)
    // ------------------------------------------------------------------

    function testClaimRefundArbitrationRulingOneBypassesDeadline() public {
        // Create a Job WITH a hook mounted.
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(hook), 0, address(0));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(provider);
        job.grabJob(jobId, AGENT_PROVIDER);
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));

        // Buyer disputes (deadline NOT reached yet).
        vm.prank(buyer);
        hook.dispute(jobId, keccak256("bad work"));

        // Resolver rules in favour of buyer (ruling=1).
        vm.prank(evaluator);
        hook.resolveDispute(jobId, 1);

        // Buyer refunds after the announcement period — deadline bypassed
        // (escrow settled in FULL via executeArbitrationResult).
        vm.warp(block.timestamp + job.ANNOUNCEMENT_PERIOD() + 1);
        uint256 buyerBefore = token.balanceOf(buyer);
        job.executeArbitrationResult(jobId);
        assertEq(token.balanceOf(buyer), buyerBefore + FUND_AMOUNT);
    }

    function testClaimRefundArbitrationRulingTwoBlocksRefundBeforeDeadline() public {
        // ruling=2 (provider wins) does NOT bypass the deadline.
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(hook), 0, address(0));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(provider);
        job.grabJob(jobId, AGENT_PROVIDER);
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));

        vm.prank(buyer);
        hook.dispute(jobId, keccak256("bad work"));
        vm.prank(evaluator);
        hook.resolveDispute(jobId, 2);

        // Before deadline: refund still blocked.
        vm.prank(buyer);
        vm.expectRevert("PrismSettle: bad state");
        job.claimRefund(jobId);
    }

    // ------------------------------------------------------------------
    // submit triggers Hook.onSubmitted
    // ------------------------------------------------------------------

    function testSubmitTriggersHookOnSubmitted() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(hook), 0, address(0));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(provider);
        job.grabJob(jobId, AGENT_PROVIDER);

        // After submit, the Hook should allow dispute (onSubmitted was called).
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));

        // Dispute should succeed — proves onSubmitted ran and the Hook is wired.
        vm.prank(buyer);
        hook.dispute(jobId, keccak256("reason"));
        (ArbitrationHook.HookState hs,,) = hook.getHookState(jobId);
        assertEq(uint256(hs), uint256(ArbitrationHook.HookState.Disputed));
    }

    // ------------------------------------------------------------------
    // Shard isolation
    // ------------------------------------------------------------------

    function testShardIsolationByJobId() public {
        // Two jobs from the same buyer land in shards determined by their
        // jobId hashes. We verify they are independently addressable and
        // do not collide in storage.
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.startPrank(buyer);
        uint256 id1 = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        uint256 id2 = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        vm.stopPrank();

        uint8 shard1 = uint8(id1 & 0xFF);
        uint8 shard2 = uint8(id2 & 0xFF);

        // Both jobs exist and are readable via their respective shards.
        (PrismSettleJob.JobState s1, , , , , , , , , ) = job.getJobState(id1);
        (PrismSettleJob.JobState s2, , , , , , , , , ) = job.getJobState(id2);
        assertEq(uint256(s1), uint256(PrismSettleJob.JobState.Created));
        assertEq(uint256(s2), uint256(PrismSettleJob.JobState.Created));

        // Funding one does not affect the other.
        vm.prank(buyer);
        job.fundViaToken(id1, FUND_AMOUNT, "");
        (, , , uint256 amt1, , , , , , ) = job.getJobState(id1);
        (, , , uint256 amt2, , , , , , ) = job.getJobState(id2);
        assertEq(amt1, FUND_AMOUNT);
        assertEq(amt2, 0);
    }

    // ------------------------------------------------------------------
    // Full lifecycle (happy path)
    // ------------------------------------------------------------------

    function testFullLifecycleCreateFundAssignSubmitComplete() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));

        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");

        vm.prank(provider);
        job.grabJob(jobId, AGENT_PROVIDER);

        vm.prank(provider);
        job.submit(jobId, keccak256("deliverable"), keccak256("proof"));

        uint256 provBefore = token.balanceOf(provider);
        vm.prank(buyer);
        job.complete(jobId, uint96(0.9e18));

        assertEq(token.balanceOf(provider), provBefore + FUND_AMOUNT);
        (PrismSettleJob.JobState s, , , , , , , , , ) = job.getJobState(jobId);
        assertEq(uint256(s), uint256(PrismSettleJob.JobState.Completed));
    }

    // ------------------------------------------------------------------
    // Arbitration (deposit-based fee model)
    // ------------------------------------------------------------------

    /// @notice ruling=1: buyer gets the FULL escrow refund; the provider's
    ///         deposit (fixed 5%) covers the arbitrator fee.
    function testArbitrationRulingOneProviderDepositCoversFee() public {
        address feeRecipient = address(0xFEE);
        uint256 feeBps = 500; // decorative: deposit fixed at 5%
        uint256 jobId = _createFundAssignSubmitWithHook(feeBps, feeRecipient);

        vm.prank(buyer);
        hook.dispute(jobId, keccak256("dispute"));

        uint256 feeBefore = token.balanceOf(feeRecipient);
        vm.prank(evaluator);
        hook.resolveDispute(jobId, 1);
        assertEq(token.balanceOf(feeRecipient), feeBefore + DEPOSIT, "arbitrator fee = provider deposit");

        vm.warp(block.timestamp + job.ANNOUNCEMENT_PERIOD() + 1);
        uint256 buyerBefore = token.balanceOf(buyer);
        job.executeArbitrationResult(jobId);
        assertEq(token.balanceOf(buyer), buyerBefore + FUND_AMOUNT, "full escrow refund");
        assertEq(token.balanceOf(address(job)), 0, "contract drained");
    }

    /// @notice Registered feeBps (10%) does not change the fixed 5% deposit.
    function testArbitrationRulingOneTenPercentFee() public {
        address feeRecipient = address(0xFEE);
        uint256 feeBps = 1000; // decorative
        uint256 jobId = _createFundAssignSubmitWithHook(feeBps, feeRecipient);

        vm.prank(buyer);
        hook.dispute(jobId, keccak256("dispute"));

        uint256 feeBefore = token.balanceOf(feeRecipient);
        vm.prank(evaluator);
        hook.resolveDispute(jobId, 1);
        assertEq(token.balanceOf(feeRecipient), feeBefore + DEPOSIT, "deposit fixed at 5%");

        vm.warp(block.timestamp + job.ANNOUNCEMENT_PERIOD() + 1);
        uint256 buyerBefore = token.balanceOf(buyer);
        job.executeArbitrationResult(jobId);
        assertEq(token.balanceOf(buyer), buyerBefore + FUND_AMOUNT, "full escrow refund");
    }

    /// @notice ruling=2: provider gets the FULL escrow; buyer's deposit
    ///         covers the arbitrator fee.
    function testArbitrationRulingTwoBuyerDepositCoversFee() public {
        address feeRecipient = address(0xFEE);
        uint256 feeBps = 500;
        uint256 jobId = _createFundAssignSubmitWithHook(feeBps, feeRecipient);

        vm.prank(buyer);
        hook.dispute(jobId, keccak256("dispute"));

        uint256 feeBefore = token.balanceOf(feeRecipient);
        vm.prank(evaluator);
        hook.resolveDispute(jobId, 2);
        assertEq(token.balanceOf(feeRecipient), feeBefore + DEPOSIT, "arbitrator fee = buyer deposit");

        vm.warp(block.timestamp + job.ANNOUNCEMENT_PERIOD() + 1);
        uint256 provBefore = token.balanceOf(provider);
        job.executeArbitrationResult(jobId);
        assertEq(token.balanceOf(provider), provBefore + FUND_AMOUNT, "full escrow payout");
        assertEq(token.balanceOf(address(job)), 0, "contract drained");
    }

    /// @notice Registered feeBps (10%) does not change the fixed 5% deposit.
    function testArbitrationRulingTwoTenPercentFee() public {
        address feeRecipient = address(0xFEE);
        uint256 feeBps = 1000;
        uint256 jobId = _createFundAssignSubmitWithHook(feeBps, feeRecipient);

        vm.prank(buyer);
        hook.dispute(jobId, keccak256("dispute"));

        uint256 feeBefore = token.balanceOf(feeRecipient);
        vm.prank(evaluator);
        hook.resolveDispute(jobId, 2);
        assertEq(token.balanceOf(feeRecipient), feeBefore + DEPOSIT, "deposit fixed at 5%");

        vm.warp(block.timestamp + job.ANNOUNCEMENT_PERIOD() + 1);
        uint256 provBefore = token.balanceOf(provider);
        job.executeArbitrationResult(jobId);
        assertEq(token.balanceOf(provider), provBefore + FUND_AMOUNT, "full escrow payout");
    }

    /// @notice Escrow refunded in FULL — arbitrator fee covered by provider deposit.
    function testArbitrationRulingOneRefundsBuyerAfterAnnouncement() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(hook), 0, address(0));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(provider);
        job.grabJob(jobId, AGENT_PROVIDER);
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));

        vm.prank(buyer);
        hook.dispute(jobId, keccak256("bad work"));

        uint256 feeBefore = token.balanceOf(address(0xFEE));
        vm.prank(evaluator);
        hook.resolveDispute(jobId, 1);
        assertEq(token.balanceOf(address(0xFEE)), feeBefore + DEPOSIT, "arbitrator fee = provider deposit");

        vm.warp(block.timestamp + job.ANNOUNCEMENT_PERIOD() + 1);
        uint256 buyerBefore = token.balanceOf(buyer);
        job.executeArbitrationResult(jobId);
        assertEq(token.balanceOf(buyer), buyerBefore + FUND_AMOUNT, "full escrow refund");
    }

    /// @notice ruling=2: provider gets the FULL escrow payout after announcement.
    function testArbitrationRulingTwoPaysProviderAfterAnnouncement() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(hook), 0, address(0));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(provider);
        job.grabJob(jobId, AGENT_PROVIDER);
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));

        vm.prank(buyer);
        hook.dispute(jobId, keccak256("bad work"));

        uint256 feeBefore = token.balanceOf(address(0xFEE));
        vm.prank(evaluator);
        hook.resolveDispute(jobId, 2);
        assertEq(token.balanceOf(address(0xFEE)), feeBefore + DEPOSIT, "arbitrator fee = buyer deposit");

        vm.expectRevert("PrismSettle: announcement period not passed");
        job.executeArbitrationResult(jobId);

        vm.warp(block.timestamp + job.ANNOUNCEMENT_PERIOD() + 1);
        uint256 provBefore = token.balanceOf(provider);
        job.executeArbitrationResult(jobId);
        assertEq(token.balanceOf(provider), provBefore + FUND_AMOUNT, "full escrow payout");
    }

    // ------------------------------------------------------------------
    // Anti self-dealing
    // ------------------------------------------------------------------

    function testGrabJobRevertsSelfDealing() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");

        vm.prank(buyer);
        vm.expectRevert("PrismSettle: self-dealing");
        job.grabJob(jobId, AGENT_PROVIDER);
    }

    // ------------------------------------------------------------------
    // grabJob — autonomy (only the agent owner may claim jobs)
    // ------------------------------------------------------------------

    function testGrabJobRevertsUnauthorizedCaller() public {
        uint256 jobId = _createAndFund();
        address stranger = address(0xBEEF);
        vm.prank(stranger);
        vm.expectRevert("PrismSettle: not owner");
        job.grabJob(jobId, AGENT_PROVIDER);
    }

    // ------------------------------------------------------------------
    // reject → resubmit loop with deposits
    // ------------------------------------------------------------------

    /// @notice Buyer rejects (posts deposit) → provider resubmits → buyer
    ///         completes → the reject deposit is released back.
    function testRejectAndResubmitThenComplete() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(hook), 0, address(0));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(provider);
        job.grabJob(jobId, AGENT_PROVIDER);
        vm.prank(provider);
        job.submit(jobId, keccak256("v1"), keccak256("p1"));

        uint256 buyerBefore = token.balanceOf(buyer);
        vm.prank(buyer);
        job.reject(jobId, keccak256("needs more detail"));
        assertEq(token.balanceOf(buyer), buyerBefore - DEPOSIT, "reject deposit deducted");

        (,,, uint256 bd, uint256 pd) = hook.hookData(jobId);
        assertEq(bd, DEPOSIT, "buyer reject deposit recorded");
        assertEq(pd, 0, "no provider deposit yet");

        vm.prank(provider);
        job.submit(jobId, keccak256("v2"), keccak256("p2"));

        uint256 buyerMid = token.balanceOf(buyer);
        vm.prank(buyer);
        job.complete(jobId, uint96(0.9e18));
        assertEq(token.balanceOf(buyer), buyerMid + DEPOSIT, "deposit released on complete");
    }

    /// @notice Buyer rejects (deposit) then loses a provider dispute → ALL
    ///         buyer deposits (reject + dispute) are forfeited to the arbitrator.
    function testRejectDepositForfeitedOnBuyerLoss() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(hook), 0, address(0));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(provider);
        job.grabJob(jobId, AGENT_PROVIDER);
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));

        vm.prank(buyer);
        job.reject(jobId, keccak256("not good enough"));
        vm.prank(provider);
        hook.dispute(jobId, keccak256("work is complete"));

        (,,, uint256 bd,) = hook.hookData(jobId);
        assertEq(bd, 2 * DEPOSIT, "reject + dispute deposits");

        uint256 feeBefore = token.balanceOf(address(0xFEE));
        vm.prank(evaluator);
        hook.resolveDispute(jobId, 2);
        assertEq(token.balanceOf(address(0xFEE)), feeBefore + 2 * DEPOSIT, "buyer deposits forfeited");
    }

    // ------------------------------------------------------------------
    // Per-job payment token (multi-currency: USDC + WMON coexist)
    // ------------------------------------------------------------------

    /// @notice A job created with a custom token (WMON) funds, pays and
    ///         settles entirely in that token; the default USDC is untouched.
    function testFundWithCustomTokenSettlesInThatToken() public {
        MockERC20 wmon = new MockERC20("Wrapped MON", "WMON");
        wmon.mint(buyer, 1000 ether);
        vm.startPrank(buyer);
        wmon.approve(address(job), type(uint256).max);
        wmon.approve(address(hook), type(uint256).max);
        vm.stopPrank();
        vm.prank(provider);
        wmon.approve(address(hook), type(uint256).max);

        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(hook), 0, address(wmon));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        assertEq(wmon.balanceOf(address(job)), FUND_AMOUNT, "escrow in WMON");
        assertEq(token.balanceOf(address(job)), 0, "no USDC escrow");

        vm.prank(provider);
        job.grabJob(jobId, AGENT_PROVIDER);
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));

        uint256 provBefore = wmon.balanceOf(provider);
        vm.prank(buyer);
        job.complete(jobId, uint96(0.9e18));
        assertEq(wmon.balanceOf(provider), provBefore + FUND_AMOUNT, "payout in WMON");
        assertEq(wmon.balanceOf(address(job)), 0, "WMON drained");
        assertEq(token.balanceOf(address(job)), 0, "USDC untouched");
    }

    /// @notice Dispute deposits for a WMON job are pulled/released in WMON
    ///         (Hook resolves the job token via getJobPaymentToken).
    function testDisputeDepositsUseJobToken() public {
        MockERC20 wmon = new MockERC20("Wrapped MON", "WMON");
        wmon.mint(buyer, 1000 ether);
        wmon.mint(provider, 1000 ether);
        vm.startPrank(buyer);
        wmon.approve(address(job), type(uint256).max);
        wmon.approve(address(hook), type(uint256).max);
        vm.stopPrank();
        vm.prank(provider);
        wmon.approve(address(hook), type(uint256).max);

        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(hook), 0, address(wmon));
        vm.prank(buyer);
        job.fundViaToken(jobId, FUND_AMOUNT, "");
        vm.prank(provider);
        job.grabJob(jobId, AGENT_PROVIDER);
        vm.prank(provider);
        job.submit(jobId, keccak256("d"), keccak256("p"));

        vm.prank(buyer);
        hook.dispute(jobId, keccak256("dispute"));

        (,,, uint256 bd, uint256 pd) = hook.hookData(jobId);
        assertEq(bd, DEPOSIT, "buyer deposit in WMON");
        assertEq(pd, DEPOSIT, "provider deposit in WMON");
        assertEq(wmon.balanceOf(address(hook)), 2 * DEPOSIT, "Hook holds WMON deposits");
        assertEq(token.balanceOf(address(hook)), 0, "no USDC deposits pulled");
    }

    /// @notice x402 receipts carry no token info: funding a custom-token job
    ///         through the facilitator reverts (default token only).
    function testX402RejectsCustomTokenJob() public {
        MockERC20 wmon = new MockERC20("Wrapped MON", "WMON");
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = job.createJob(AGENT_ID, 0, deadline, address(0), 0, address(wmon));

        bytes memory receipt = bytes("x402-receipt-v1");
        vm.prank(buyer);
        vm.expectRevert("PrismSettle: token mismatch");
        job.fundViaToken(jobId, 0, receipt);
    }

}
