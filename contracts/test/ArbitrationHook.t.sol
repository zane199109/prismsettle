// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Test} from "forge-std/Test.sol";
import {IAccessControl} from "@openzeppelin/contracts/access/IAccessControl.sol";
import {ArbitrationHook} from "../src/ArbitrationHook.sol";
import {MockERC20} from "../src/mocks/MockERC20.sol";

/// @title MockJobForHook
/// @notice Minimal stand-in for PrismSettleJob so the Hook test can verify
///         the Job↔Hook back-reference without pulling in the full Job
///         contract (which needs ERC-20 wiring). Implements the views the
///         Hook calls during dispute() / resolveDispute() / deposits.
contract MockJobForHook {
    mapping(uint256 => address) public buyers;
    mapping(uint256 => address) public providers;
    mapping(uint256 => uint256) public submittedAts;
    mapping(uint256 => uint256) public amounts;
    mapping(uint256 => address) public paymentTokens;

    function setBuyer(uint256 jobId, address buyer) external {
        buyers[jobId] = buyer;
    }

    function setProvider(uint256 jobId, address provider) external {
        providers[jobId] = provider;
    }

    function setSubmittedAt(uint256 jobId, uint256 ts) external {
        submittedAts[jobId] = ts;
    }

    function setAmount(uint256 jobId, uint256 amount) external {
        amounts[jobId] = amount;
    }

    function setPaymentToken(uint256 jobId, address tk) external {
        paymentTokens[jobId] = tk;
    }

    function getJobBuyer(uint256 jobId) external view returns (address) {
        return buyers[jobId];
    }

    function getJobProvider(uint256 jobId) external view returns (address) {
        return providers[jobId];
    }

    function getJobSubmittedAt(uint256 jobId) external view returns (uint256) {
        return submittedAts[jobId];
    }

    function getJobAmount(uint256 jobId) external view returns (uint256) {
        return amounts[jobId];
    }

    function getJobPaymentToken(uint256 jobId) external view returns (address) {
        return paymentTokens[jobId];
    }

    /// @notice No-op for testing; the Hook calls this after resolveDispute.
    function notifyDisputeResolved(uint256, uint8) external {}
}

/// @title MockRegistryForHook
/// @notice Minimal stand-in for PrismSettleRegistry so the Hook can query
///         reputation scores during arbitrator selection.
contract MockRegistryForHook {
    mapping(uint256 => uint256) public scores;

    function setScore(uint256 agentId, uint256 score) external {
        scores[agentId] = score;
    }

    function getScore(uint256 agentId) external view returns (uint256) {
        return scores[agentId];
    }
}

/// @title ArbitrationHookTest
/// @notice Covers SD §3.4: dispute lifecycle, arbitrator pool management,
///         reputation-weighted selection, resolver rulings, and access
///         control.
contract ArbitrationHookTest is Test {
    ArbitrationHook internal hook;
    MockJobForHook internal mockJob;
    MockRegistryForHook internal mockRegistry;
    MockERC20 internal token;

    address internal deployer = address(this);
    address internal buyer = address(0xB0B);
    address internal provider = address(0x0B0B);
    address internal resolver = address(0xCAFE);
    address internal nonBuyer = address(0xE1A1);
    address internal arbitrator1 = address(0xAAA1);
    address internal arbitrator2 = address(0xAAA2);
    address internal nonArbitrator = address(0xBBB1);

    uint256 internal constant JOB_ID = 42;
    uint256 internal constant AGENT_ID_1 = 101;
    uint256 internal constant AGENT_ID_2 = 102;
    uint256 internal constant AMOUNT = 100e18;
    uint256 internal constant DEPOSIT = 5e18; // AMOUNT × DEPOSIT_BPS(500) / 10000
    bytes32 internal constant REASON = keccak256("bad deliverable");

    function setUp() public {
        token = new MockERC20("MockUSDC", "USDC");
        hook = new ArbitrationHook(address(token));
        mockJob = new MockJobForHook();
        mockRegistry = new MockRegistryForHook();

        // Wire back-references: Hook → MockJob + MockRegistry.
        hook.setJobContract(address(mockJob));
        hook.setRegistryContract(address(mockRegistry));

        // Grant resolver role to the Evaluator address (fallback).
        hook.grantRole(hook.RESOLVER_ROLE(), resolver);

        // Job state for JOB_ID: buyer, provider, amount, recent submit.
        mockJob.setBuyer(JOB_ID, buyer);
        mockJob.setProvider(JOB_ID, provider);
        mockJob.setSubmittedAt(JOB_ID, block.timestamp);
        mockJob.setAmount(JOB_ID, AMOUNT);
        mockJob.setPaymentToken(JOB_ID, address(token));

        // Fund both parties and approve the Hook for deposits.
        token.mint(buyer, 1000e18);
        token.mint(provider, 1000e18);
        vm.prank(buyer);
        token.approve(address(hook), type(uint256).max);
        vm.prank(provider);
        token.approve(address(hook), type(uint256).max);

        // Register two arbitrators with different scores.
        mockRegistry.setScore(AGENT_ID_1, 0.7e18);
        mockRegistry.setScore(AGENT_ID_2, 0.9e18);

        vm.prank(arbitrator1);
        hook.registerArbitrator(AGENT_ID_1, 500, address(0xFEE1));
        vm.prank(arbitrator2);
        hook.registerArbitrator(AGENT_ID_2, 300, address(0xFEE2));
    }

    // ------------------------------------------------------------------
    // setJobContract
    // ------------------------------------------------------------------

    function testSetJobContractSucceedsOnce() public {
        vm.expectRevert("PrismSettle: job already set");
        hook.setJobContract(address(mockJob));
    }

    function testRevertSetJobContractIfNotAdmin() public {
        ArbitrationHook fresh = new ArbitrationHook(address(token));
        bytes32 adminRole = fresh.DEFAULT_ADMIN_ROLE();
        vm.prank(nonBuyer);
        vm.expectRevert(
            abi.encodeWithSelector(IAccessControl.AccessControlUnauthorizedAccount.selector, nonBuyer, adminRole)
        );
        fresh.setJobContract(address(mockJob));
    }

    // ------------------------------------------------------------------
    // setRegistryContract
    // ------------------------------------------------------------------

    function testSetRegistryContractSucceedsOnce() public {
        vm.expectRevert("PrismSettle: registry already set");
        hook.setRegistryContract(address(mockRegistry));
    }

    function testRevertSetRegistryContractIfNotAdmin() public {
        ArbitrationHook fresh = new ArbitrationHook(address(token));
        bytes32 adminRole = fresh.DEFAULT_ADMIN_ROLE();
        vm.prank(nonBuyer);
        vm.expectRevert(
            abi.encodeWithSelector(IAccessControl.AccessControlUnauthorizedAccount.selector, nonBuyer, adminRole)
        );
        fresh.setRegistryContract(address(mockRegistry));
    }

    // ------------------------------------------------------------------
    // registerArbitrator
    // ------------------------------------------------------------------

    /// @notice Register an arbitrator with a valid fee.
    function testRegisterArbitratorSucceeds() public {
        address newArb = address(0xCCC1);
        mockRegistry.setScore(201, 0.7e18);
        vm.prank(newArb);
        vm.expectEmit(true, false, false, true);
        emit ArbitrationHook.ArbitratorRegistered(newArb, 201, 400, address(0xFEE3));
        hook.registerArbitrator(201, 400, address(0xFEE3));

        assertEq(hook.arbitratorCount(), 3);
        assertEq(hook.activeArbitratorCount(), 3);
    }

    /// @notice Cannot register the same address twice.
    function testRevertRegisterArbitratorIfAlreadyRegistered() public {
        vm.prank(arbitrator1);
        vm.expectRevert("Arbitration: already registered");
        hook.registerArbitrator(AGENT_ID_1, 500, address(0xFEE1));
    }

    /// @notice Fee exceeding 20% must revert.
    function testRevertRegisterArbitratorFeeExceedsMax() public {
        address newArb = address(0xCCC2);
        vm.prank(newArb);
        vm.expectRevert("Arbitration: fee exceeds max");
        hook.registerArbitrator(1, 2001, address(0xFEE));
    }

    /// @notice Zero recipient must revert.
    function testRevertRegisterArbitratorZeroRecipient() public {
        address newArb = address(0xCCC3);
        vm.prank(newArb);
        vm.expectRevert("Arbitration: zero recipient");
        hook.registerArbitrator(1, 500, address(0));
    }

    // ------------------------------------------------------------------
    // unregisterArbitrator
    // ------------------------------------------------------------------

    /// @notice Unregister removes the arbitrator from the active pool.
    function testUnregisterArbitratorSucceeds() public {
        assertEq(hook.activeArbitratorCount(), 2);

        vm.prank(arbitrator1);
        vm.expectEmit(true, false, false, false);
        emit ArbitrationHook.ArbitratorUnregistered(arbitrator1);
        hook.unregisterArbitrator();

        assertEq(hook.activeArbitratorCount(), 1);
        // arbitratorList length stays the same (soft delete).
        assertEq(hook.arbitratorCount(), 2);
    }

    function testRevertUnregisterArbitratorIfNotRegistered() public {
        vm.prank(nonArbitrator);
        vm.expectRevert("Arbitration: not registered");
        hook.unregisterArbitrator();
    }

    // ------------------------------------------------------------------
    // onSubmitted
    // ------------------------------------------------------------------

    function testRevertOnSubmittedIfNotJobContract() public {
        vm.prank(nonBuyer);
        vm.expectRevert("PrismSettle: not job");
        hook.onSubmitted(JOB_ID);
    }

    function testOnSubmittedSucceedsFromJobContract() public {
        vm.prank(address(mockJob));
        hook.onSubmitted(JOB_ID);
        (ArbitrationHook.HookState s,,) = hook.getHookState(JOB_ID);
        assertEq(uint256(s), uint256(ArbitrationHook.HookState.None));
    }

    // ------------------------------------------------------------------
    // dispute — selects highest-reputation arbitrator
    // ------------------------------------------------------------------

    /// @notice Dispute transitions to Disputed and selects the highest-
    ///         reputation arbitrator (arbitrator2 has 0.9e18 > 0.7e18).
    function testDisputeSelectsHighestReputationArbitrator() public {
        vm.prank(buyer);
        vm.expectEmit(true, false, false, true);
        emit ArbitrationHook.Disputed(JOB_ID, REASON);
        hook.dispute(JOB_ID, REASON);

        (ArbitrationHook.HookState s, bytes32 reason, uint8 ruling) = hook.getHookState(JOB_ID);
        assertEq(uint256(s), uint256(ArbitrationHook.HookState.Disputed));
        assertEq(reason, REASON);
        assertEq(ruling, 0);

        // Verify the highest-reputation arbitrator was selected.
        address selected = hook.disputeArbitrator(JOB_ID);
        assertEq(selected, arbitrator2, "should select arbitrator with highest score");
    }

    /// @notice Arbitrator with lower score can be selected when the higher-
    ///         scored one is unregistered.
    function testDisputeSelectsNextBestAfterUnregister() public {
        vm.prank(arbitrator2);
        hook.unregisterArbitrator();

        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        address selected = hook.disputeArbitrator(JOB_ID);
        assertEq(selected, arbitrator1, "should select next best after unregister");
    }

    function testRevertDisputeIfNotParty() public {
        vm.prank(nonBuyer);
        vm.expectRevert("PrismSettle: not job party");
        hook.dispute(JOB_ID, REASON);
    }

    function testRevertDisputeIfAlreadyDisputed() public {
        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        vm.prank(buyer);
        vm.expectRevert("PrismSettle: already disputed");
        hook.dispute(JOB_ID, REASON);
    }

    function testRevertDisputeIfNoArbitratorAvailable() public {
        vm.prank(arbitrator1);
        hook.unregisterArbitrator();
        vm.prank(arbitrator2);
        hook.unregisterArbitrator();

        vm.prank(buyer);
        vm.expectRevert("PrismSettle: no arbitrator available");
        hook.dispute(JOB_ID, REASON);
    }

    // ------------------------------------------------------------------
    // resolveDispute — selected arbitrator only
    // ------------------------------------------------------------------

    function testResolveDisputeBySelectedArbitrator() public {
        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        // arbitrator2 was selected (highest score).
        vm.prank(arbitrator2);
        vm.expectEmit(true, false, false, true);
        emit ArbitrationHook.DisputeResolved(JOB_ID, 1, arbitrator2);
        hook.resolveDispute(JOB_ID, 1);

        (ArbitrationHook.HookState s,, uint8 ruling) = hook.getHookState(JOB_ID);
        assertEq(uint256(s), uint256(ArbitrationHook.HookState.DisputeResolved));
        assertEq(ruling, 1);
    }

    function testResolveDisputeByResolverFallback() public {
        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        // RESOLVER_ROLE can also resolve (fallback).
        vm.prank(resolver);
        hook.resolveDispute(JOB_ID, 2);

        (,, uint8 ruling) = hook.getHookState(JOB_ID);
        assertEq(ruling, 2);
    }

    function testRevertResolveDisputeByNonSelectedArbitrator() public {
        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        // arbitrator1 was NOT selected (arbitrator2 has higher score).
        vm.prank(arbitrator1);
        vm.expectRevert("PrismSettle: not authorized");
        hook.resolveDispute(JOB_ID, 1);
    }

    function testRevertResolveDisputeRulingZero() public {
        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        vm.prank(arbitrator2);
        vm.expectRevert("PrismSettle: ruling must be 1 or 2");
        hook.resolveDispute(JOB_ID, 0);
    }

    function testRevertResolveDisputeIfNotDisputed() public {
        vm.prank(resolver);
        vm.expectRevert("PrismSettle: not disputed");
        hook.resolveDispute(JOB_ID, 1);
    }

    // ------------------------------------------------------------------
    // getArbitratorFeeConfig
    // ------------------------------------------------------------------

    /// @notice Returns the fee config of the selected arbitrator.
    function testGetArbitratorFeeConfigAfterDispute() public {
        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        (uint256 feeBps, address recipient) = hook.getArbitratorFeeConfig(JOB_ID);
        assertEq(feeBps, 300, "arbitrator2 fee is 300 bps");
        assertEq(recipient, address(0xFEE2), "arbitrator2 fee recipient");
    }

    /// @notice Returns zero before a dispute is opened.
    function testGetArbitratorFeeConfigDefault() public {
        (uint256 feeBps, address recipient) = hook.getArbitratorFeeConfig(JOB_ID);
        assertEq(feeBps, 0);
        assertEq(recipient, address(0));
    }

    // ------------------------------------------------------------------
    // getHookState default
    // ------------------------------------------------------------------

    function testGetHookStateDefaultIsNone() public {
        (ArbitrationHook.HookState s, bytes32 reason, uint8 ruling) = hook.getHookState(999);
        assertEq(uint256(s), uint256(ArbitrationHook.HookState.None));
        assertEq(reason, bytes32(0));
        assertEq(ruling, 0);
    }

    // ------------------------------------------------------------------
    // activeArbitratorCount
    // ------------------------------------------------------------------

    function testActiveArbitratorCount() public {
        assertEq(hook.activeArbitratorCount(), 2);

        // Unregister one → count drops to 1.
        vm.prank(arbitrator1);
        hook.unregisterArbitrator();
        assertEq(hook.activeArbitratorCount(), 1);

        // Register a new one → count goes to 2.
        address newArb = address(0xDDD1);
        mockRegistry.setScore(301, 0.7e18);
        vm.prank(newArb);
        hook.registerArbitrator(301, 100, address(0xFEE4));
        assertEq(hook.activeArbitratorCount(), 2);
    }

    // ------------------------------------------------------------------
    // Provider dispute + dispute window
    // ------------------------------------------------------------------

    /// @notice The provider (not just the buyer) may open a dispute.
    function testDisputeByProviderSucceeds() public {
        vm.prank(provider);
        hook.dispute(JOB_ID, REASON);
        (ArbitrationHook.HookState s,,) = hook.getHookState(JOB_ID);
        assertEq(uint256(s), uint256(ArbitrationHook.HookState.Disputed));
    }

    /// @notice Dispute is only allowed within DISPUTE_WINDOW of the latest submit.
    function testRevertDisputeAfterWindow() public {
        vm.warp(block.timestamp + hook.DISPUTE_WINDOW() + 1);
        vm.prank(buyer);
        vm.expectRevert("PrismSettle: dispute window closed");
        hook.dispute(JOB_ID, REASON);
    }

    // ------------------------------------------------------------------
    // Deposit lifecycle
    // ------------------------------------------------------------------

    /// @notice recordReject pulls a buyer deposit; releaseDeposits returns it.
    function testReleaseDepositsReturnsFunds() public {
        vm.prank(address(mockJob));
        hook.recordReject(JOB_ID);

        (,,, uint256 bd,,) = hook.hookData(JOB_ID);
        assertEq(bd, DEPOSIT, "reject deposit recorded");

        uint256 buyerBefore = token.balanceOf(buyer);
        vm.prank(address(mockJob));
        hook.releaseDeposits(JOB_ID);
        assertEq(token.balanceOf(buyer), buyerBefore + DEPOSIT, "deposit released");
        (,,, uint256 bd2,,) = hook.hookData(JOB_ID);
        assertEq(bd2, 0, "deposits cleared");
    }

    /// @notice dispute pulls deposits from BOTH parties.
    function testDisputeCollectsBothDeposits() public {
        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        (,,, uint256 bd, uint256 pd,) = hook.hookData(JOB_ID);
        assertEq(bd, DEPOSIT, "buyer dispute deposit");
        assertEq(pd, DEPOSIT, "provider dispute deposit");
    }
}