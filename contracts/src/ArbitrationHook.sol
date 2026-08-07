// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/// @title ArbitrationHook
/// @notice Dispute resolution hook with an arbitrator pool. Each arbitrator
///         registers with their own fee rate (like a lawyer's hourly rate).
///         When a dispute is raised, the hook selects the arbitrator with the
///         highest on-chain reputation score from the PrismSettleRegistry.
///         Only the selected arbitrator may resolve the dispute, and they
///         receive the fee.
///
/// @dev Lifecycle:
///      None → Disputed (buyer calls dispute) → DisputeResolved (selected
///      arbitrator calls resolveDispute with ruling 1=buyer / 2=provider).
///      ruling=0 is invalid and reverts.
///
///      Back-reference: the Hook needs to query PrismSettleJob.getJobBuyer
///      to verify the caller of dispute() is the Job's buyer. Because Job
///      also needs the Hook address in its constructor, we deploy Hook
///      first with jobContract=address(0), then call setJobContract() once
///      after Job is deployed.
interface IPrismSettleJobView {
    /// @notice Read-only view of a Job's buyer.
    function getJobBuyer(uint256 jobId) external view returns (address);
    /// @notice Read-only view of a Job's provider (for dispute rights).
    function getJobProvider(uint256 jobId) external view returns (address);
    /// @notice Timestamp of the latest submit; anchors the dispute window.
    function getJobSubmittedAt(uint256 jobId) external view returns (uint256);
    /// @notice Effective payment token (per-job or contract default).
    function getJobPaymentToken(uint256 jobId) external view returns (address);
    /// @notice Escrow amount (for deposit computation).
    function getJobAmount(uint256 jobId) external view returns (uint256);
}
/// @title IPrismSettleJobNotify
/// @notice Interface for ArbitrationHook to notify the Job contract after
///         a dispute resolution, so the Job can transition to DisputeResolved
///         state and start the announcement period.
interface IPrismSettleJobNotify {
    /// @notice Called by the Hook after resolveDispute.
    /// @param jobId The disputed job.
    /// @param ruling 1=buyer wins, 2=provider wins.
    function notifyDisputeResolved(uint256 jobId, uint8 ruling) external;
}

/// @title IPrismSettleRegistryView
/// @notice Minimal view interface to query an agent's reputation score from
///         the PrismSettleRegistry. Only the getScore function is needed for
///         arbitrator selection.
interface IPrismSettleRegistryView {
    /// @notice Get an agent's current score with inactivity decay applied.
    function getScore(uint256 agentId) external view returns (uint256);
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
        bytes32 disputeReason;
        uint8 disputeRuling;
        HookState state;
        uint256 buyerDeposit; // cumulative reject/dispute deposits paid by buyer
        uint256 providerDeposit; // dispute deposit paid by provider
    }

    /// @dev Per-arbitrator configuration. Each arbitrator sets their own
    ///      fee rate and recipient address, like a lawyer's rate card.
    struct ArbitratorConfig {
        uint256 agentId;       // PrismSettleRegistry agentId for reputation
        uint256 feeBps;        // Basis points (e.g. 500 = 5%), capped at 20%
        address feeRecipient;  // Address that receives the fee
        bool registered;
    }

    /// @dev Resolver role for resolveDispute (Evaluator fallback).
    bytes32 public constant RESOLVER_ROLE = keccak256("RESOLVER_ROLE");

    /// @dev Maximum evaluator fee in basis points (20%). No single
    ///      arbitrator can charge more than 20% of the disputed amount.
    uint256 public constant MAX_EVALUATOR_FEE_BPS = 2000;

    /// @dev Dispute window: either party may dispute within this long after
    ///      the latest submit. After it closes, the buyer may complete or
    ///      claim a refund without fear of a late dispute.
    uint256 public constant DISPUTE_WINDOW = 24 hours;

    /// @dev Deposit rate for reject/dispute: amount × 5% (basis points).
    ///      The losing party's deposit(s) cover the arbitrator's fee; the
    ///      winning party gets their deposit back and the full escrow.
    uint256 public constant DEPOSIT_BPS = 500;

    // ---------------------------------------------------------------------
    // Storage
    // ---------------------------------------------------------------------

    /// @dev jobId → HookData.
    mapping(uint256 => HookData) public hookData;

    /// @dev jobId → address of the selected arbitrator for this dispute.
    mapping(uint256 => address) public disputeArbitrator;

    /// @dev Back-reference to the Job contract.
    address public jobContract;

    /// @dev Back-reference to the PrismSettleRegistry for reputation queries.
    address public registryContract;

    /// @dev Contract-default payment token (deposits are pulled in the
    ///      JOB's token via getJobPaymentToken; this is only the fallback
    ///      reference kept for the constructor).
    IERC20 public paymentToken;

    /// @dev Dynamic list of registered arbitrator addresses. Used for
    ///      iteration when selecting the best arbitrator.
    address[] public arbitratorList;

    /// @dev address → ArbitratorConfig.
    mapping(address => ArbitratorConfig) public arbitratorConfigs;

    // ---------------------------------------------------------------------
    // Events
    // ---------------------------------------------------------------------

    event Disputed(uint256 indexed jobId, bytes32 reasonHash);
    event DisputeResolved(uint256 indexed jobId, uint8 ruling, address indexed arbitrator);
    event ArbitratorRegistered(address indexed arbitrator, uint256 agentId, uint256 feeBps, address feeRecipient);
    event ArbitratorUnregistered(address indexed arbitrator);
    event ArbitratorSelected(uint256 indexed jobId, address indexed arbitrator, uint256 score);
    event RejectDepositPaid(uint256 indexed jobId, address indexed buyer, uint256 amount);
    event DepositsReleased(uint256 indexed jobId, uint256 buyerAmount, uint256 providerAmount);
    event DepositsSettled(uint256 indexed jobId, uint8 ruling, uint256 forfeitedAmount, address forfeitedBy);

    // ---------------------------------------------------------------------
    // Constructor
    // ---------------------------------------------------------------------

    constructor(address token_) {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        require(token_ != address(0), "Arbitration: zero token");
        paymentToken = IERC20(token_);
    }

    // ---------------------------------------------------------------------
    // One-shot initialization
    // ---------------------------------------------------------------------

    /// @notice Set the Job contract address. May only be called once.
    function setJobContract(address job) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(jobContract == address(0), "PrismSettle: job already set");
        require(job != address(0), "PrismSettle: zero job");
        jobContract = job;
    }

    /// @notice Set the PrismSettleRegistry contract address for reputation
    ///         queries. May only be called once.
    function setRegistryContract(address registry) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(registryContract == address(0), "PrismSettle: registry already set");
        require(registry != address(0), "PrismSettle: zero registry");
        registryContract = registry;
    }

    /// @notice Effective ERC-20 for a job's deposits: the job's own payment
    ///         token (per-job multi-currency support).
    function _jobToken(uint256 jobId) internal view returns (IERC20) {
        return IERC20(IPrismSettleJobView(jobContract).getJobPaymentToken(jobId));
    }

    // ---------------------------------------------------------------------
    // Arbitrator pool management
    // ---------------------------------------------------------------------

    /// @notice Register as an arbitrator. Each address may register once.
    /// @param agentId      PrismSettleRegistry agentId for reputation-based selection.
    /// @param feeBps       Fee in basis points (e.g. 500 = 5%). Capped at 20%.
    /// @param feeRecipient Address that receives the fee when a dispute is resolved.
    function registerArbitrator(uint256 agentId, uint256 feeBps, address feeRecipient) external {
        require(feeBps <= MAX_EVALUATOR_FEE_BPS, "Arbitration: fee exceeds max");
        require(feeRecipient != address(0), "Arbitration: zero recipient");
        require(!arbitratorConfigs[msg.sender].registered, "Arbitration: already registered");
        require(registryContract != address(0), "Arbitration: registry not set");
        require(
            IPrismSettleRegistryView(registryContract).getScore(agentId) >= 0.7e18,
            "Arbitration: reputation too low"
        );

        arbitratorConfigs[msg.sender] = ArbitratorConfig({
            agentId: agentId,
            feeBps: feeBps,
            feeRecipient: feeRecipient,
            registered: true
        });
        arbitratorList.push(msg.sender);
        emit ArbitratorRegistered(msg.sender, agentId, feeBps, feeRecipient);
    }

    /// @notice Unregister from the arbitrator pool. The address is marked
    ///         as unregistered but remains in the list (soft delete).
    function unregisterArbitrator() external {
        require(arbitratorConfigs[msg.sender].registered, "Arbitration: not registered");
        arbitratorConfigs[msg.sender].registered = false;
        emit ArbitratorUnregistered(msg.sender);
    }

    /// @notice Return the total number of registered arbitrators.
    function arbitratorCount() external view returns (uint256) {
        return arbitratorList.length;
    }

    // ---------------------------------------------------------------------
    // Job → Hook callbacks
    // ---------------------------------------------------------------------

    /// @notice Called by the Job contract when a provider submits work.
    function onSubmitted(uint256 jobId) external {
        require(msg.sender == jobContract, "PrismSettle: not job");
        HookData storage h = hookData[jobId];
        require(h.state == HookState.None, "PrismSettle: already disputed");
    }

    /// @notice Called by the Job contract when the buyer rejects a deliverable.
    ///         The buyer posts a deposit (amount × DEPOSIT_BPS) per reject —
    ///         an economic brake on infinite rework loops. Deposits are
    ///         released if the job settles without arbitration, or forfeited
    ///         to the arbitrator if the buyer loses a dispute.
    function recordReject(uint256 jobId) external {
        require(msg.sender == jobContract, "PrismSettle: not job");
        HookData storage h = hookData[jobId];
        require(h.state == HookState.None, "PrismSettle: already disputed");

        address buyer = IPrismSettleJobView(jobContract).getJobBuyer(jobId);
        uint256 amount = IPrismSettleJobView(jobContract).getJobAmount(jobId);
        uint256 deposit = (amount * DEPOSIT_BPS) / 10000;
        require(deposit > 0, "Arbitration: zero deposit");
        require(_jobToken(jobId).transferFrom(buyer, address(this), deposit), "Arbitration: deposit failed");
        h.buyerDeposit += deposit;
        emit RejectDepositPaid(jobId, buyer, deposit);
    }

    /// @notice Called by the Job contract when a job settles without
    ///         arbitration (complete / claimRefund). Returns all deposits.
    function releaseDeposits(uint256 jobId) external {
        require(msg.sender == jobContract, "PrismSettle: not job");
        HookData storage h = hookData[jobId];
        require(h.state != HookState.Disputed, "Arbitration: pending dispute");

        address buyer = IPrismSettleJobView(jobContract).getJobBuyer(jobId);
        address provider = IPrismSettleJobView(jobContract).getJobProvider(jobId);
        uint256 b = h.buyerDeposit;
        uint256 p = h.providerDeposit;
        h.buyerDeposit = 0;
        h.providerDeposit = 0;
        if (b > 0) {
            require(_jobToken(jobId).transfer(buyer, b), "Arbitration: buyer deposit release failed");
        }
        if (p > 0) {
            require(_jobToken(jobId).transfer(provider, p), "Arbitration: provider deposit release failed");
        }
        emit DepositsReleased(jobId, b, p);
    }

    // ---------------------------------------------------------------------
    // Buyer → Hook dispute
    // ---------------------------------------------------------------------

    /// @notice Open a dispute for a Job. Automatically selects the
    ///         highest-reputation arbitrator from the pool. Either the
    ///         buyer (unsatisfied with the deliverable) or the provider
    ///         (buyer not confirming completion) may open a dispute within
    ///         DISPUTE_WINDOW of the latest submit.
    /// @param jobId     The Job being disputed.
    /// @param reasonHash Hash of the dispute reason.
    function dispute(uint256 jobId, bytes32 reasonHash) external {
        HookData storage h = hookData[jobId];
        require(h.state == HookState.None, "PrismSettle: already disputed");
        require(jobContract != address(0), "PrismSettle: job not set");
        require(registryContract != address(0), "PrismSettle: registry not set");

        address buyer = IPrismSettleJobView(jobContract).getJobBuyer(jobId);
        address provider = IPrismSettleJobView(jobContract).getJobProvider(jobId);
        require(msg.sender == buyer || msg.sender == provider, "PrismSettle: not job party");

        // Dispute window: only within DISPUTE_WINDOW of the latest submit.
        uint256 submittedAt = IPrismSettleJobView(jobContract).getJobSubmittedAt(jobId);
        require(submittedAt != 0 && block.timestamp <= submittedAt + DISPUTE_WINDOW, "PrismSettle: dispute window closed");

        // Select the highest-reputation arbitrator.
        address selected = _selectArbitrator();
        require(selected != address(0), "PrismSettle: no arbitrator available");

        // Both parties post a deposit (amount × DEPOSIT_BPS). The loser's
        // deposit(s) pay the arbitrator; the winner gets theirs back and the
        // full escrow. Reject deposits already paid accumulate on the buyer.
        uint256 amount = IPrismSettleJobView(jobContract).getJobAmount(jobId);
        uint256 deposit = (amount * DEPOSIT_BPS) / 10000;
        require(deposit > 0, "Arbitration: zero deposit");
        require(_jobToken(jobId).transferFrom(buyer, address(this), deposit), "Arbitration: buyer deposit failed");
        h.buyerDeposit += deposit;
        require(_jobToken(jobId).transferFrom(provider, address(this), deposit), "Arbitration: provider deposit failed");
        h.providerDeposit += deposit;

        h.disputeReason = reasonHash;
        h.state = HookState.Disputed;
        disputeArbitrator[jobId] = selected;

        emit Disputed(jobId, reasonHash);
        emit ArbitratorSelected(jobId, selected, _getArbitratorScore(selected));
    }

    // ---------------------------------------------------------------------
    // Resolver → Hook ruling
    // ---------------------------------------------------------------------

    /// @notice Resolve a dispute. Only the selected arbitrator may call,
    ///         or anyone with RESOLVER_ROLE (evaluator fallback).
    /// @param jobId The Job under dispute.
    /// @param ruling 1=buyer wins (refund), 2=provider wins (complete).
    function resolveDispute(uint256 jobId, uint8 ruling) external {
        require(ruling == 1 || ruling == 2, "PrismSettle: ruling must be 1 or 2");
        HookData storage h = hookData[jobId];
        require(h.state == HookState.Disputed, "PrismSettle: not disputed");
        require(
            msg.sender == disputeArbitrator[jobId] || hasRole(RESOLVER_ROLE, msg.sender),
            "PrismSettle: not authorized"
        );

        h.disputeRuling = ruling;
        h.state = HookState.DisputeResolved;
        emit DisputeResolved(jobId, ruling, msg.sender);

        // Settle deposits: loser's deposit(s) go to the arbitrator, the
        // winner's are returned. Escrow itself is settled in full by the
        // Job contract (executeArbitrationResult) — no fee deducted there.
        address buyer = IPrismSettleJobView(jobContract).getJobBuyer(jobId);
        address provider = IPrismSettleJobView(jobContract).getJobProvider(jobId);
        address feeRecipient = arbitratorConfigs[disputeArbitrator[jobId]].feeRecipient;
        uint256 forfeited;
        if (ruling == 1) {
            // Buyer wins: provider's deposit is forfeited.
            forfeited = h.providerDeposit;
            if (forfeited > 0) {
                require(_jobToken(jobId).transfer(feeRecipient, forfeited), "Arbitration: fee transfer failed");
            }
            if (h.buyerDeposit > 0) {
                require(_jobToken(jobId).transfer(buyer, h.buyerDeposit), "Arbitration: deposit refund failed");
            }
        } else {
            // Provider wins: buyer's deposit(s) forfeited.
            forfeited = h.buyerDeposit;
            if (forfeited > 0) {
                require(_jobToken(jobId).transfer(feeRecipient, forfeited), "Arbitration: fee transfer failed");
            }
            if (h.providerDeposit > 0) {
                require(_jobToken(jobId).transfer(provider, h.providerDeposit), "Arbitration: deposit refund failed");
            }
        }
        h.buyerDeposit = 0;
        h.providerDeposit = 0;
        emit DepositsSettled(jobId, ruling, forfeited, ruling == 1 ? provider : buyer);

        // Notify the Job contract to transition to DisputeResolved state.
        IPrismSettleJobNotify(jobContract).notifyDisputeResolved(jobId, ruling);
    }

    // ---------------------------------------------------------------------
    // Views — fee resolution for the Job contract
    // ---------------------------------------------------------------------

    /// @notice Get the fee configuration for the arbitrator selected for a
    ///         given job. Used by PrismSettleJob._resolveEvaluatorFee().
    /// @return feeBps     Fee in basis points.
    /// @return recipient  Fee recipient address.
    function getArbitratorFeeConfig(uint256 jobId) external view returns (uint256 feeBps, address recipient) {
        address selected = disputeArbitrator[jobId];
        if (selected == address(0)) return (0, address(0));
        ArbitratorConfig storage config = arbitratorConfigs[selected];
        return (config.feeBps, config.feeRecipient);
    }

    // ---------------------------------------------------------------------
    // Views
    // ---------------------------------------------------------------------

    /// @notice Read the dispute state for a Job.
    function getHookState(uint256 jobId) external view returns (HookState state, bytes32 reasonHash, uint8 ruling) {
        HookData storage h = hookData[jobId];
        return (h.state, h.disputeReason, h.disputeRuling);
    }

    /// @notice Get the number of registered arbitrators (active only).
    function activeArbitratorCount() external view returns (uint256) {
        uint256 count;
        for (uint256 i = 0; i < arbitratorList.length; i++) {
            if (arbitratorConfigs[arbitratorList[i]].registered) {
                count++;
            }
        }
        return count;
    }

    // ---------------------------------------------------------------------
    // Internal
    // ---------------------------------------------------------------------

    /// @notice Select the highest-reputation arbitrator from the pool.
    /// @return The address of the selected arbitrator, or address(0) if none.
    function _selectArbitrator() internal view returns (address) {
        address best;
        uint256 bestScore;
        for (uint256 i = 0; i < arbitratorList.length; i++) {
            address candidate = arbitratorList[i];
            if (!arbitratorConfigs[candidate].registered) continue;
            uint256 score = _getArbitratorScore(candidate);
            if (score > bestScore) {
                bestScore = score;
                best = candidate;
            }
        }
        return best;
    }

    /// @notice Get the reputation score for an arbitrator from the Registry.
    function _getArbitratorScore(address arbitrator) internal view returns (uint256) {
        uint256 agentId = arbitratorConfigs[arbitrator].agentId;
        return IPrismSettleRegistryView(registryContract).getScore(agentId);
    }
}