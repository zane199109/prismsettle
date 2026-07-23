// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

/// @title ArbitrationHook
/// @notice Dispute resolution hook attached to a PrismSettleJob contract.
///         When a Job has a non-zero hook address, the buyer may open a
///         dispute after the provider submits, and a resolver (Evaluator)
///         issues a ruling that overrides the normal deadline-gated refund.
///
/// @dev Lifecycle:
///      None → Disputed (buyer calls dispute) → DisputeResolved (resolver
///      calls resolveDispute with ruling 1=buyer / 2=provider).
///      ruling=0 is invalid and reverts.
///
///      Back-reference: the Hook needs to query PrismSettleJob.getJobState
///      to verify the caller of dispute() is the Job's buyer. Because Job
///      also needs the Hook address in its constructor, we deploy Hook
///      first with jobContract=address(0), then call setJobContract() once
///      after Job is deployed.
interface IPrismSettleJobView {
    /// @notice Read-only view of a Job's buyer. Defined here to avoid a
    ///         circular import with PrismSettleJob.sol.
    function getJobBuyer(uint256 jobId) external view returns (address);
}

contract ArbitrationHook is AccessControl {
    // ---------------------------------------------------------------------
    // Types & constants
    // ---------------------------------------------------------------------

    enum HookState {
        None,
        Disputed,
        DisputeResolved
    }

    struct HookData {
        bytes32 disputeReason; // BUYER arbitration reason hash
        uint8 disputeRuling; // 0=not ruled 1=BUYER 2=Provider
        HookState state;
    }

    /// @dev Resolver role for resolveDispute (Evaluator holds this).
    bytes32 public constant RESOLVER_ROLE = keccak256("RESOLVER_ROLE");

    // ---------------------------------------------------------------------
    // Storage
    // ---------------------------------------------------------------------

    /// @dev jobId → HookData. Jobs without a dispute return state=None.
    mapping(uint256 => HookData) public hookData;

    /// @dev Back-reference to the Job contract for buyer verification.
    ///      Set once via setJobContract; address(0) until then.
    address public jobContract;

    // ---------------------------------------------------------------------
    // Events
    // ---------------------------------------------------------------------

    event Disputed(uint256 indexed jobId, bytes32 reasonHash);
    event DisputeResolved(uint256 indexed jobId, uint8 ruling);

    // ---------------------------------------------------------------------
    // Constructor
    // ---------------------------------------------------------------------

    constructor() {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
    }

    // ---------------------------------------------------------------------
    // One-shot back-reference initialization
    // ---------------------------------------------------------------------

    /// @notice Set the Job contract address. May only be called once.
    /// @dev    Deployment order: deploy Hook → deploy Job(hook) →
    ///         Hook.setJobContract(job). After this, onSubmitted / dispute
    ///         can verify callers against the Job's state.
    function setJobContract(address job) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(jobContract == address(0), "PrismSettle: job already set");
        require(job != address(0), "PrismSettle: zero job");
        jobContract = job;
    }

    // ---------------------------------------------------------------------
    // Job → Hook callbacks
    // ---------------------------------------------------------------------

    /// @notice Called by the Job contract when a provider submits work.
    ///         Records the entry so dispute() can later be called.
    /// @dev    Only the Job contract may call this. If jobId already has
    ///         a None state, this is a no-op write (state stays None but
    ///         the entry is created). In practice this just ensures the
    ///         jobId is known; dispute() is what transitions None→Disputed.
    function onSubmitted(uint256 jobId) external {
        require(msg.sender == jobContract, "PrismSettle: not job");
        // No state transition here; dispute() is the buyer's explicit action.
        // We intentionally do not mutate hookData — getHookState returns
        // HookState.None for uninitialised entries, which is correct.
        // This callback exists so future Hook variants can react to submit.
        HookData storage h = hookData[jobId];
        require(h.state == HookState.None, "PrismSettle: already disputed");
    }

    // ---------------------------------------------------------------------
    // Buyer → Hook dispute
    // ---------------------------------------------------------------------

    /// @notice Open a dispute for a Job. Only the Job's buyer may call.
    /// @param jobId     The Job being disputed.
    /// @param reasonHash Hash of the dispute reason (off-chain evidence ref).
    function dispute(uint256 jobId, bytes32 reasonHash) external {
        HookData storage h = hookData[jobId];
        require(h.state == HookState.None, "PrismSettle: already disputed");
        // Verify msg.sender is the Job's buyer by querying the Job contract.
        require(jobContract != address(0), "PrismSettle: job not set");
        address buyer = IPrismSettleJobView(jobContract).getJobBuyer(jobId);
        require(msg.sender == buyer, "PrismSettle: not buyer");
        h.disputeReason = reasonHash;
        h.state = HookState.Disputed;
        emit Disputed(jobId, reasonHash);
    }

    // ---------------------------------------------------------------------
    // Resolver → Hook ruling
    // ---------------------------------------------------------------------

    /// @notice Resolve a dispute. Only RESOLVER_ROLE (Evaluator) may call.
    /// @param jobId The Job under dispute.
    /// @param ruling 1=buyer wins (refund), 2=provider wins (complete).
    ///               ruling=0 is invalid and reverts.
    function resolveDispute(uint256 jobId, uint8 ruling) external onlyRole(RESOLVER_ROLE) {
        require(ruling == 1 || ruling == 2, "PrismSettle: ruling must be 1 or 2");
        HookData storage h = hookData[jobId];
        require(h.state == HookState.Disputed, "PrismSettle: not disputed");
        h.disputeRuling = ruling;
        h.state = HookState.DisputeResolved;
        emit DisputeResolved(jobId, ruling);
    }

    // ---------------------------------------------------------------------
    // View
    // ---------------------------------------------------------------------

    /// @notice Read the dispute state for a Job.
    /// @return state       HookState enum.
    /// @return reasonHash  The dispute reason hash (0 if no dispute).
    /// @return ruling      0=not ruled, 1=buyer, 2=provider.
    function getHookState(uint256 jobId) external view returns (HookState state, bytes32 reasonHash, uint8 ruling) {
        HookData storage h = hookData[jobId];
        return (h.state, h.disputeReason, h.disputeRuling);
    }
}
