// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import {Test} from "forge-std/Test.sol";
import {IAccessControl} from "@openzeppelin/contracts/access/IAccessControl.sol";
import {ArbitrationHook} from "../src/ArbitrationHook.sol";

/// @title MockJobForHook
/// @notice Minimal stand-in for PrismSettleJob so the Hook test can verify
///         the Job↔Hook back-reference without pulling in the full Job
///         contract (which needs ERC-20 wiring). Implements only
///         getJobBuyer, which the Hook calls during dispute().
contract MockJobForHook {
    mapping(uint256 => address) public buyers;

    function setBuyer(uint256 jobId, address buyer) external {
        buyers[jobId] = buyer;
    }

    function getJobBuyer(uint256 jobId) external view returns (address) {
        return buyers[jobId];
    }
}

/// @title ArbitrationHookTest
/// @notice Covers SD §3.4: dispute lifecycle, resolver rulings, access
///         control, and the Job↔Hook back-reference initialization.
contract ArbitrationHookTest is Test {
    ArbitrationHook internal hook;
    MockJobForHook internal mockJob;

    address internal deployer = address(this);
    address internal buyer = address(0xB0B);
    address internal resolver = address(0xCAFE);
    address internal nonBuyer = address(0xE1A1);

    uint256 internal constant JOB_ID = 42;
    bytes32 internal constant REASON = keccak256("bad deliverable");

    function setUp() public {
        hook = new ArbitrationHook();
        mockJob = new MockJobForHook();

        // Wire the back-reference: Hook → MockJob.
        hook.setJobContract(address(mockJob));
        // Grant resolver role to the Evaluator address.
        hook.grantRole(hook.RESOLVER_ROLE(), resolver);

        // Seed a buyer for JOB_ID in the mock Job.
        mockJob.setBuyer(JOB_ID, buyer);
    }

    // ------------------------------------------------------------------
    // setJobContract
    // ------------------------------------------------------------------

    function testSetJobContractSucceedsOnce() public {
        // setUp already called setJobContract; a second call by the admin
        // must revert with "job already set" (not an access-control error,
        // because the admin is authorised — the one-shot guard fires first).
        vm.expectRevert("PrismSettle: job already set");
        hook.setJobContract(address(mockJob));
    }

    function testRevertSetJobContractIfNotAdmin() public {
        ArbitrationHook fresh = new ArbitrationHook();
        bytes32 adminRole = fresh.DEFAULT_ADMIN_ROLE();
        vm.prank(nonBuyer);
        vm.expectRevert(
            abi.encodeWithSelector(IAccessControl.AccessControlUnauthorizedAccount.selector, nonBuyer, adminRole)
        );
        fresh.setJobContract(address(mockJob));
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
        // MockJob is the registered jobContract; prank as it.
        vm.prank(address(mockJob));
        hook.onSubmitted(JOB_ID);
        // State remains None — onSubmitted is a no-op transition.
        (ArbitrationHook.HookState s,,) = hook.getHookState(JOB_ID);
        assertEq(uint256(s), uint256(ArbitrationHook.HookState.None));
    }

    // ------------------------------------------------------------------
    // dispute
    // ------------------------------------------------------------------

    function testDisputeTransitionsToDisputed() public {
        vm.prank(buyer);
        vm.expectEmit(true, false, false, true);
        emit ArbitrationHook.Disputed(JOB_ID, REASON);
        hook.dispute(JOB_ID, REASON);

        (ArbitrationHook.HookState s, bytes32 reason, uint8 ruling) = hook.getHookState(JOB_ID);
        assertEq(uint256(s), uint256(ArbitrationHook.HookState.Disputed));
        assertEq(reason, REASON);
        assertEq(ruling, 0);
    }

    function testRevertDisputeIfNotBuyer() public {
        vm.prank(nonBuyer);
        vm.expectRevert("PrismSettle: not buyer");
        hook.dispute(JOB_ID, REASON);
    }

    function testRevertDisputeIfAlreadyDisputed() public {
        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        vm.prank(buyer);
        vm.expectRevert("PrismSettle: already disputed");
        hook.dispute(JOB_ID, REASON);
    }

    // ------------------------------------------------------------------
    // resolveDispute
    // ------------------------------------------------------------------

    function testResolveDisputeRulingOneTransitionsToResolved() public {
        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        vm.prank(resolver);
        vm.expectEmit(true, false, false, true);
        emit ArbitrationHook.DisputeResolved(JOB_ID, 1);
        hook.resolveDispute(JOB_ID, 1);

        (ArbitrationHook.HookState s,, uint8 ruling) = hook.getHookState(JOB_ID);
        assertEq(uint256(s), uint256(ArbitrationHook.HookState.DisputeResolved));
        assertEq(ruling, 1);
    }

    function testResolveDisputeRulingTwoTransitionsToResolved() public {
        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        vm.prank(resolver);
        hook.resolveDispute(JOB_ID, 2);

        (,, uint8 ruling) = hook.getHookState(JOB_ID);
        assertEq(ruling, 2);
    }

    function testRevertResolveDisputeRulingZero() public {
        // FR-J08: ruling=0 is invalid and must revert.
        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        vm.prank(resolver);
        vm.expectRevert("PrismSettle: ruling must be 1 or 2");
        hook.resolveDispute(JOB_ID, 0);
    }

    function testRevertResolveDisputeIfNotResolver() public {
        vm.prank(buyer);
        hook.dispute(JOB_ID, REASON);

        bytes32 resolverRole = hook.RESOLVER_ROLE();
        vm.prank(nonBuyer);
        vm.expectRevert(
            abi.encodeWithSelector(IAccessControl.AccessControlUnauthorizedAccount.selector, nonBuyer, resolverRole)
        );
        hook.resolveDispute(JOB_ID, 1);
    }

    function testRevertResolveDisputeIfNotDisputed() public {
        // JOB_ID has not been disputed; resolving must fail.
        vm.prank(resolver);
        vm.expectRevert("PrismSettle: not disputed");
        hook.resolveDispute(JOB_ID, 1);
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
}
