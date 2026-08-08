// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {IX402Facilitator} from "./interfaces/IX402Facilitator.sol";
import {ArbitrationHook} from "./ArbitrationHook.sol";

/// @title IRegistryView
/// @notice Minimal Registry interface for reputation queries used by grabJob.
interface IRegistryView {
    function getScore(uint256 agentId) external view returns (uint256);
    function agentOwner(uint256 agentId) external view returns (address);
}

/// @title IRegistryWriter
/// @notice Registry write interface for Buyer rating via complete().
interface IRegistryWriter {
    function submitValidation(uint256 agentId, uint96 score, bytes32 proofHash, uint256 jobId, uint8 source) external;
}

/// @title PrismSettleJob
/// @notice ERC-8183 Job lifecycle contract: create → fund → grab → submit →
///         complete (or refund). An ArbitrationHook lets the buyer dispute
///         and a resolver override the refund with an announcement period.
///
/// @dev Storage layout:
///      Jobs are sharded by the low 8 bits of jobId to spread write
///      contention on Monad OCC. jobId is derived from (buyer, nonce)
///      so different buyers never collide on createJob.
contract PrismSettleJob is AccessControl {
    // ---------------------------------------------------------------------
    // Types & constants
    // ---------------------------------------------------------------------

    /// @dev ERC-8183 four macro-states map onto seven concrete states.
    ///      Open=Created, Funded=Funded, Submitted=Assigned→Submitted,
    ///      Terminal=DisputeResolved→Completed/Refunded.
    enum JobState {
        Created,
        Funded,
        Assigned,
        Submitted,
        DisputeResolved, // arbitrator ruled, awaiting announcement period
        Completed,
        Refunded
    }

    struct Job {
        address buyer;
        address provider;
        uint256 providerAgentId; // Registry agentId of the provider
        uint256 amount; // locked escrow amount (in the job's payment token)
        bytes32 deliverableHash; // IPFS hash or content hash
        bytes32 proofHash; // execution proof hash (formal compliance)
        uint64 deadline; // timeout refund deadline
        JobState state;
        uint256 parentJobId; // secondary dispatch hook (V1: interface only)
        address hook; // ArbitrationHook address (0 = not mounted)
        address paymentToken; // per-job token; address(0) = contract default
        uint64 createdAt;
        uint96 minProviderReputation; // minimum reputation score for provider
        uint256 disputeResolvedAt; // timestamp when dispute was resolved
        uint256 submittedAt; // timestamp of the latest submit (dispute window anchor)
    }

    /// @dev Evaluator role for complete().
    bytes32 public constant COMMERCE_EVALUATOR_ROLE = keccak256("COMMERCE_EVALUATOR_ROLE");

    /// @dev Announcement period after dispute resolution before funds can
    ///      be released. Configurable via constructor / setAnnouncementPeriod
    ///      (60s demo default; shorten for fast demo days).
    uint256 public announcementPeriod;

    // ---------------------------------------------------------------------
    // Storage
    // ---------------------------------------------------------------------

    /// @dev shardJobs[jobId & 0xFF][jobId] — sharded by jobId, not agentId,
    ///      because createJob's write contention is per-buyer (nonce), and
    ///      jobId embeds the buyer. Different buyers → different jobIds →
    ///      different shards → no OCC abort on concurrent createJob.
    mapping(uint8 => mapping(uint256 => Job)) public shardJobs;

    /// @dev Account-level nonce for jobId generation. Downgrades the
    ///      createJob write lock from a global hotspot to per-account.
    mapping(address => uint256) public buyerNonce;

    /// @dev x402 receipt anti-replay. Even if the Facilitator
    ///      has its own replay protection, the contract double-checks.
    mapping(bytes32 => bool) public usedReceipts;

    /// @dev Testnet USDC (or MockERC20 on anvil).
    IERC20 public paymentToken;

    /// @dev x402 Facilitator; address(0) disables the receipt path.
    address public facilitator;

    /// @dev PrismSettleRegistry address for reputation queries in grabJob.
    address public registry;

    // ---------------------------------------------------------------------
    // Events
    // ---------------------------------------------------------------------

    event JobCreated(uint256 indexed agentId, uint256 indexed jobId, address buyer, uint64 deadline, address hook, uint96 minProviderReputation, address paymentToken);
    event Funded(uint256 indexed jobId, address buyer, uint256 amount);
    event Assigned(uint256 indexed jobId, address provider);
    event Submitted(uint256 indexed jobId, bytes32 deliverableHash, bytes32 proofHash);
    event Completed(uint256 indexed jobId, address provider, uint256 amount);
    event Refunded(uint256 indexed jobId, address buyer, uint256 amount);
    event DisputeResolvedAnnounced(uint256 indexed jobId, uint8 ruling, uint256 resolvedAt, uint256 releaseAt);
    event ArbitrationExecuted(uint256 indexed jobId, uint8 ruling, uint256 amount);
    event Rejected(uint256 indexed jobId, address buyer, bytes32 reasonHash);
    event AnnouncementPeriodUpdated(uint256 period);

    // ---------------------------------------------------------------------
    // Constructor
    // ---------------------------------------------------------------------

    constructor(address token, address hookFacilitator, uint256 announcementPeriod_) {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        paymentToken = IERC20(token);
        facilitator = hookFacilitator; // may be address(0)
        announcementPeriod = announcementPeriod_ == 0 ? 60 : announcementPeriod_;
    }

    /// @notice Adjust the post-resolution announcement period (admin only).
    function setAnnouncementPeriod(uint256 period) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(period > 0, "PrismSettle: zero period");
        announcementPeriod = period;
        emit AnnouncementPeriodUpdated(period);
    }

    // ---------------------------------------------------------------------
    // Admin: set Registry
    // ---------------------------------------------------------------------

    /// @notice Set the PrismSettleRegistry address for reputation queries.
    function setRegistry(address registryAddr) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(registryAddr != address(0), "PrismSettle: zero registry");
        registry = registryAddr;
    }

    // ---------------------------------------------------------------------
    // createJob
    // ---------------------------------------------------------------------

    /// @notice Create a new Job in the Created state.
    /// @param agentId              Target agent (emitted in event for indexer; not stored).
    /// @param parentJobId          Secondary dispatch parent (V1: interface only, 0 ok).
    /// @param deadline             Unix timestamp after which buyer may claimRefund.
    /// @param hook                 ArbitrationHook address (0 = no arbitration).
    /// @param minProviderReputation Minimum reputation score for provider grabJob.
    /// @param token                Per-job payment token (ERC-20); address(0) =
    ///                             contract default token. Any ERC-20 (USDC,
    ///                             WMON, ...) is supported — multi-currency escrow.
    function createJob(uint256 agentId, uint256 parentJobId, uint64 deadline, address hook, uint96 minProviderReputation, address token)
        external
        returns (uint256 jobId)
    {
        require(deadline > block.timestamp, "PrismSettle: deadline passed");
        uint256 nonce = buyerNonce[msg.sender]++; 
        jobId = uint256(keccak256(abi.encodePacked(msg.sender, nonce)));
        uint8 shard = uint8(jobId & 0xFF);
        shardJobs[shard][jobId] = Job({
            buyer: msg.sender,
            provider: address(0),
            providerAgentId: 0,
            amount: 0,
            deliverableHash: bytes32(0),
            proofHash: bytes32(0),
            deadline: deadline,
            state: JobState.Created,
            parentJobId: parentJobId,
            hook: hook,
            paymentToken: token,
            createdAt: uint64(block.timestamp),
            minProviderReputation: minProviderReputation,
            disputeResolvedAt: 0,
            submittedAt: 0
        });
        emit JobCreated(agentId, jobId, msg.sender, deadline, hook, minProviderReputation, token);
    }

    // ---------------------------------------------------------------------
    // fundViaToken — x402 / ERC-20 split
    // ---------------------------------------------------------------------

    /// @notice Fund a Created Job. Routes to x402 Facilitator when a
    ///         receipt is provided, otherwise pulls ERC-20 directly.
    /// @param jobId       Job to fund.
    /// @param amount      ERC-20 amount (must be 0 when receipt present).
    /// @param x402Receipt Opaque x402 receipt bytes; empty → ERC-20 path.
    function fundViaToken(uint256 jobId, uint256 amount, bytes calldata x402Receipt) external {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        require(j.state == JobState.Created, "PrismSettle: bad state");
        require(msg.sender == j.buyer, "PrismSettle: not buyer");

        uint256 funded;
        if (x402Receipt.length > 0) {
            require(facilitator != address(0), "PrismSettle: no facilitator");
            require(amount == 0, "PrismSettle: amount must be 0 with receipt");
            // x402 receipts carry no token info — only the contract-default
            // token can be settled through the facilitator.
            require(
                j.paymentToken == address(0) || j.paymentToken == address(paymentToken),
                "PrismSettle: token mismatch"
            );
            bytes32 receiptHash = keccak256(x402Receipt);
            require(!usedReceipts[receiptHash], "PrismSettle: receipt used");
            usedReceipts[receiptHash] = true;
            funded = IX402Facilitator(facilitator).settleWithReceipt(j.buyer, address(this), x402Receipt);
        } else {
            require(amount > 0, "PrismSettle: zero amount");
            address tk = j.paymentToken == address(0) ? address(paymentToken) : j.paymentToken;
            require(IERC20(tk).transferFrom(j.buyer, address(this), amount), "PrismSettle: transferFrom failed");
            funded = amount;
        }
        j.amount = funded;
        j.state = JobState.Funded;
        emit Funded(jobId, j.buyer, funded);
    }

    // ---------------------------------------------------------------------
    // grabJob — provider self-assigns
    // ---------------------------------------------------------------------

    /// @notice Provider grabs a Funded job. The contract checks the
    ///         provider's agent reputation against the job's minimum
    ///         requirement, and that the caller is the agent's owner —
    ///         agents are autonomous (their wallet is their operator);
    ///         nobody else may claim a job on their behalf.
    /// @param jobId            The job to grab.
    /// @param providerAgentId  The provider's agentId in the Registry.
    function grabJob(uint256 jobId, uint256 providerAgentId) external {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        require(j.state == JobState.Funded, "PrismSettle: bad state");
        require(j.provider == address(0), "PrismSettle: already assigned");
        // Anti self-dealing: a buyer must not grab their own job, otherwise
        // completion rewards could be farmed by buying from themselves.
        require(msg.sender != j.buyer, "PrismSettle: self-dealing");
        require(registry != address(0), "PrismSettle: registry not set");

        // Check provider reputation against minimum requirement.
        uint256 score = IRegistryView(registry).getScore(providerAgentId);
        require(score >= uint256(j.minProviderReputation), "PrismSettle: reputation too low");

        // Ownership: only the agent's registered owner may claim jobs with
        // that agentId. Without this, anyone could grab a job with a
        // borrowed high-reputation agentId and steal the payout.
        require(msg.sender == IRegistryView(registry).agentOwner(providerAgentId), "PrismSettle: not owner");

        j.provider = msg.sender;
        j.providerAgentId = providerAgentId;
        j.state = JobState.Assigned;
        emit Assigned(jobId, msg.sender);
    }

    // ---------------------------------------------------------------------
    // submit
    // ---------------------------------------------------------------------

    /// @notice Provider submits work. First submission moves Assigned →
    ///         Submitted; a resubmission (after the buyer rejected) overwrites
    ///         the deliverable and resets the dispute window (submittedAt).
    ///         If a Hook is mounted, triggers Hook.onSubmitted so the buyer
    ///         may subsequently dispute.
    function submit(uint256 jobId, bytes32 deliverableHash, bytes32 proofHash) external {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        require(
            j.state == JobState.Assigned || j.state == JobState.Submitted,
            "PrismSettle: bad state"
        );
        require(msg.sender == j.provider, "PrismSettle: not provider");
        require(deliverableHash != bytes32(0), "PrismSettle: empty deliverable");
        require(proofHash != bytes32(0), "PrismSettle: empty proof");

        // No resubmission while a dispute is pending.
        if (j.state == JobState.Submitted && j.hook != address(0)) {
            (ArbitrationHook.HookState hs,,) = ArbitrationHook(j.hook).getHookState(jobId);
            require(hs != ArbitrationHook.HookState.Disputed, "PrismSettle: pending dispute");
        }

        j.deliverableHash = deliverableHash;
        j.proofHash = proofHash;
        j.state = JobState.Submitted;
        j.submittedAt = block.timestamp;
        emit Submitted(jobId, deliverableHash, proofHash);

        if (j.hook != address(0)) {
            ArbitrationHook(j.hook).onSubmitted(jobId);
        }
    }

    // ---------------------------------------------------------------------
    // reject — Buyer only (iterative rework loop)
    // ---------------------------------------------------------------------

    /// @notice Buyer rejects the deliverable with an opinion. The Job stays
    ///         in Submitted so the provider can resubmit an improved version,
    ///         or open a dispute if they consider the work complete.
    ///         The buyer posts a deposit per reject (anti-infinite-rework);
    ///         deposits are returned if the job settles without arbitration.
    /// @param jobId      The job to reject.
    /// @param reasonHash Hash of the buyer's opinion/rework request.
    function reject(uint256 jobId, bytes32 reasonHash) external {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        require(j.state == JobState.Submitted, "PrismSettle: bad state");
        require(msg.sender == j.buyer, "PrismSettle: not buyer");
        require(reasonHash != bytes32(0), "PrismSettle: empty reason");

        // No reject while a dispute is pending.
        if (j.hook != address(0)) {
            (ArbitrationHook.HookState hs,,) = ArbitrationHook(j.hook).getHookState(jobId);
            require(hs != ArbitrationHook.HookState.Disputed, "PrismSettle: pending dispute");
            // Buyer posts a reject deposit (requires token approval to the hook).
            ArbitrationHook(j.hook).recordReject(jobId);
        }

        emit Rejected(jobId, msg.sender, reasonHash);
    }

    // ---------------------------------------------------------------------
    // complete — Buyer only (non-dispute path)
    // ---------------------------------------------------------------------

    /// @notice Buyer marks a Submitted Job as Completed and rates the
    ///         provider. Records Buyer rating via Registry.submitValidation
    ///         with source=3 so the provider's reputation reflects buyer
    ///         satisfaction. Blocked when there is a pending dispute.
    /// @param jobId The job to complete.
    /// @param score Buyer rating for the provider (0..1e18 fixed-point).
    function complete(uint256 jobId, uint96 score) external {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        require(j.state == JobState.Submitted, "PrismSettle: bad state");
        require(msg.sender == j.buyer, "PrismSettle: not buyer");

        // Block if there is a pending dispute.
        if (j.hook != address(0)) {
            (ArbitrationHook.HookState hs,,) = ArbitrationHook(j.hook).getHookState(jobId);
            require(hs != ArbitrationHook.HookState.Disputed, "PrismSettle: pending dispute");
        }

        j.state = JobState.Completed;

        // Record Buyer rating (source=3) on the Registry.
        if (registry != address(0) && j.providerAgentId != 0) {
            IRegistryWriter(registry).submitValidation(j.providerAgentId, score, j.proofHash, jobId, 3);
        }

        // Release any reject deposits (job settled without arbitration).
        if (j.hook != address(0)) {
            ArbitrationHook(j.hook).releaseDeposits(jobId);
        }

        (uint256 fee, address recipient) = _resolveEvaluatorFee(jobId, j.amount, 2);
        uint256 payout = j.amount - fee;

        if (fee > 0) {
            require(_pay(j, recipient, fee), "PrismSettle: fee transfer failed");
        }
        require(_pay(j, j.provider, payout), "PrismSettle: payout failed");
        emit Completed(jobId, j.provider, payout);
    }

    // ---------------------------------------------------------------------
    // claimRefund (non-dispute deadline path)
    // ---------------------------------------------------------------------

    /// @notice Buyer claims a refund after the deadline.
    ///         Does NOT handle dispute bypass — dispute resolution now
    ///         goes through DisputeResolved → executeArbitrationResult.
    ///         Blocked while a dispute is pending (no refund/arbitration race).
    function claimRefund(uint256 jobId) external {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        require(
            j.state == JobState.Funded || j.state == JobState.Assigned || j.state == JobState.Submitted,
            "PrismSettle: bad state"
        );
        require(msg.sender == j.buyer, "PrismSettle: not buyer");
        require(block.timestamp >= j.deadline, "PrismSettle: deadline not reached");

        // Block while a dispute is pending (symmetric with complete()).
        if (j.hook != address(0)) {
            (ArbitrationHook.HookState hs,,) = ArbitrationHook(j.hook).getHookState(jobId);
            require(hs != ArbitrationHook.HookState.Disputed, "PrismSettle: pending dispute");
        }

        j.state = JobState.Refunded;

        // Release any reject deposits (job settled without arbitration).
        if (j.hook != address(0)) {
            ArbitrationHook(j.hook).releaseDeposits(jobId);
        }

        (uint256 fee, address recipient) = _resolveEvaluatorFee(jobId, j.amount, 1);
        uint256 refund = j.amount - fee;

        if (fee > 0) {
            require(_pay(j, recipient, fee), "PrismSettle: fee transfer failed");
        }
        require(_pay(j, j.buyer, refund), "PrismSettle: refund failed");
        emit Refunded(jobId, j.buyer, refund);
    }

    // ---------------------------------------------------------------------
    // Dispute resolution: Hook callback
    // ---------------------------------------------------------------------

    /// @notice Called by ArbitrationHook.resolveDispute to transition the
    ///         Job into DisputeResolved state and record the timestamp.
    /// @param jobId The disputed job.
    /// @param ruling 1=buyer wins, 2=provider wins.
    function notifyDisputeResolved(uint256 jobId, uint8 ruling) external {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        require(msg.sender == j.hook, "PrismSettle: not hook");
        require(j.state == JobState.Submitted, "PrismSettle: bad state");
        require(ruling == 1 || ruling == 2, "PrismSettle: invalid ruling");

        j.state = JobState.DisputeResolved;
        j.disputeResolvedAt = block.timestamp;

        uint256 releaseAt = block.timestamp + announcementPeriod;
        emit DisputeResolvedAnnounced(jobId, ruling, block.timestamp, releaseAt);
    }

    // ---------------------------------------------------------------------
    // Dispute resolution: execute arbitration result
    // ---------------------------------------------------------------------

    /// @notice Execute the arbitration result after the announcement period.
    ///         Anyone may call this after ANNOUNCEMENT_PERIOD has passed.
    ///         Ruling=1 → refund to buyer, Ruling=2 → payout to provider.
    ///         The escrow is settled in FULL to the winning party: the
    ///         arbitrator's fee is covered by the losing party's deposit(s),
    ///         which were settled in ArbitrationHook.resolveDispute.
    /// @param jobId The job to finalize.
    function executeArbitrationResult(uint256 jobId) external {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        require(j.state == JobState.DisputeResolved, "PrismSettle: bad state");
        require(block.timestamp >= j.disputeResolvedAt + announcementPeriod, "PrismSettle: announcement period not passed");
        require(j.hook != address(0), "PrismSettle: no hook");

        // Query the Hook for the ruling.
        (,, uint8 ruling) = ArbitrationHook(j.hook).getHookState(jobId);
        require(ruling == 1 || ruling == 2, "PrismSettle: invalid ruling");

        if (ruling == 1) {
            // Refund the full escrow to the buyer.
            j.state = JobState.Refunded;
            require(_pay(j, j.buyer, j.amount), "PrismSettle: refund failed");
            emit Refunded(jobId, j.buyer, j.amount);
        } else {
            // Payout the full escrow to the provider.
            j.state = JobState.Completed;
            require(_pay(j, j.provider, j.amount), "PrismSettle: payout failed");
            emit Completed(jobId, j.provider, j.amount);
        }

        emit ArbitrationExecuted(jobId, ruling, j.amount);
    }

    // ---------------------------------------------------------------------
    // Views (used by Hook and indexer)
    // ---------------------------------------------------------------------

    /// @notice Full Job state view.
    function getJobState(uint256 jobId)
        external
        view
        returns (
            JobState state,
            address buyer,
            address provider,
            uint256 amount,
            bytes32 deliverableHash,
            bytes32 proofHash,
            uint64 deadline,
            address hook,
            uint96 minProviderReputation,
            uint256 disputeResolvedAt
        )
    {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        return (
            j.state, j.buyer, j.provider, j.amount,
            j.deliverableHash, j.proofHash, j.deadline, j.hook,
            j.minProviderReputation, j.disputeResolvedAt
        );
    }

    /// @notice Buyer-only view used by ArbitrationHook.dispute to verify
    ///         the caller. Kept separate from getJobState so the Hook
    ///         does not need to decode the full tuple.
    function getJobBuyer(uint256 jobId) external view returns (address) {
        return shardJobs[uint8(jobId & 0xFF)][jobId].buyer;
    }

    /// @notice Provider view used by ArbitrationHook.dispute to allow either
    ///         party to open a dispute.
    function getJobProvider(uint256 jobId) external view returns (address) {
        return shardJobs[uint8(jobId & 0xFF)][jobId].provider;
    }

    /// @notice Timestamp of the latest submit; anchors the dispute window.
    function getJobSubmittedAt(uint256 jobId) external view returns (uint256) {
        return shardJobs[uint8(jobId & 0xFF)][jobId].submittedAt;
    }

    /// @notice Escrow amount view used by ArbitrationHook to compute
    ///         reject/dispute deposits (amount × DEPOSIT_BPS).
    function getJobAmount(uint256 jobId) external view returns (uint256) {
        return shardJobs[uint8(jobId & 0xFF)][jobId].amount;
    }

    /// @notice Effective payment token for a job: the per-job token if set,
    ///         otherwise the contract-default token. Used by the Hook to
    ///         pull/release deposits in the job's currency.
    function getJobPaymentToken(uint256 jobId) external view returns (address) {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        return j.paymentToken == address(0) ? address(paymentToken) : j.paymentToken;
    }

    // ---------------------------------------------------------------------
    // Internal — per-job token payout + evaluator fee resolution
    // ---------------------------------------------------------------------

    /// @notice Transfer `amount` of the job's payment token to `to`.
    ///         Resolves the effective token (per-job or contract default).
    function _pay(Job storage j, address to, uint256 amount) internal returns (bool) {
        address tk = j.paymentToken == address(0) ? address(paymentToken) : j.paymentToken;
        return IERC20(tk).transfer(to, amount);
    }

    /// @notice Resolve the evaluator fee for a Job that has a resolved
    ///         dispute matching the given ruling. If the Hook has no fee
    ///         configured, or the dispute ruling does not match, returns
    ///         zero fee.
    /// @param jobId  The Job to check.
    /// @param amount The total escrowed amount.
    /// @param ruling The ruling to match (1=buyer wins, 2=provider wins).
    /// @return fee       The evaluator fee amount (0 if no fee configured).
    /// @return recipient The fee recipient address (address(0) if no fee).
    function _resolveEvaluatorFee(uint256 jobId, uint256 amount, uint8 ruling)
        internal
        view
        returns (uint256 fee, address recipient)
    {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        if (j.hook == address(0)) return (0, address(0));

        // Only deduct fee when the dispute was resolved with the matching ruling.
        (,, uint8 disputeRuling) = ArbitrationHook(j.hook).getHookState(jobId);
        if (disputeRuling != ruling) return (0, address(0));

        (uint256 feeBps, address feeRecipient) = ArbitrationHook(j.hook).getArbitratorFeeConfig(jobId);

        if (feeBps == 0 || feeRecipient == address(0)) return (0, address(0));

        fee = (amount * feeBps) / 10000;
        recipient = feeRecipient;
    }
}