// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {IX402Facilitator} from "./interfaces/IX402Facilitator.sol";
import {ArbitrationHook} from "./ArbitrationHook.sol";

/// @title PrismSettleJob
/// @notice ERC-8183 Job lifecycle contract: create → fund → assign →
///         submit → complete (or refund). An optional ArbitrationHook
///         lets the buyer dispute and a resolver override the refund.
///
/// @dev Storage layout:
///      Jobs are sharded by the low 8 bits of jobId to spread write
///      contention on Monad OCC. jobId is derived from (buyer, nonce)
///      so different buyers never collide on createJob.
contract PrismSettleJob is AccessControl {
    // ---------------------------------------------------------------------
    // Types & constants
    // ---------------------------------------------------------------------

    /// @dev ERC-8183 four macro-states map onto six concrete states.
    ///      Open=Created, Funded=Funded, Submitted=Assigned→Submitted,
    ///      Terminal=Completed/Refunded. Arbitration lives in the Hook.
    enum JobState {
        Created,
        Funded,
        Assigned,
        Submitted,
        Completed,
        Refunded
    }

    struct Job {
        address buyer;
        address provider;
        uint256 amount; // locked USDC amount
        bytes32 deliverableHash; // IPFS hash or content hash
        bytes32 proofHash; // execution proof hash (formal compliance)
        uint64 deadline; // timeout refund deadline
        JobState state;
        uint256 parentJobId; // secondary dispatch hook (V1: interface only)
        address hook; // ArbitrationHook address (0 = not mounted)
        uint64 createdAt;
    }

    /// @dev Evaluator role for complete().
    bytes32 public constant COMMERCE_EVALUATOR_ROLE = keccak256("COMMERCE_EVALUATOR_ROLE");

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

    // ---------------------------------------------------------------------
    // Events
    // ---------------------------------------------------------------------

    event JobCreated(uint256 indexed agentId, uint256 indexed jobId, address buyer, uint64 deadline, address hook);
    event Funded(uint256 indexed jobId, address buyer, uint256 amount);
    event Assigned(uint256 indexed jobId, address provider);
    event Submitted(uint256 indexed jobId, bytes32 deliverableHash, bytes32 proofHash);
    event Completed(uint256 indexed jobId, address provider, uint256 amount);
    event Refunded(uint256 indexed jobId, address buyer, uint256 amount);

    // ---------------------------------------------------------------------
    // Constructor
    // ---------------------------------------------------------------------

    constructor(address token, address hookFacilitator) {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        paymentToken = IERC20(token);
        facilitator = hookFacilitator; // may be address(0)
    }

    // ---------------------------------------------------------------------
    // createJob
    // ---------------------------------------------------------------------

    /// @notice Create a new Job in the Created state.
    /// @param agentId    Target agent (emitted in event for indexer; not stored).
    /// @param parentJobId Secondary dispatch parent (V1: interface only, 0 ok).
    /// @param deadline   Unix timestamp after which buyer may claimRefund.
    /// @param hook       ArbitrationHook address (0 = no arbitration).
    function createJob(uint256 agentId, uint256 parentJobId, uint64 deadline, address hook)
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
            amount: 0,
            deliverableHash: bytes32(0),
            proofHash: bytes32(0),
            deadline: deadline,
            state: JobState.Created,
            parentJobId: parentJobId,
            hook: hook,
            createdAt: uint64(block.timestamp)
        });
        emit JobCreated(agentId, jobId, msg.sender, deadline, hook);
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
            bytes32 receiptHash = keccak256(x402Receipt);
            require(!usedReceipts[receiptHash], "PrismSettle: receipt used");
            usedReceipts[receiptHash] = true;
            funded = IX402Facilitator(facilitator).settleWithReceipt(j.buyer, address(this), x402Receipt);
        } else {
            require(amount > 0, "PrismSettle: zero amount");
            require(paymentToken.transferFrom(j.buyer, address(this), amount), "PrismSettle: transferFrom failed");
            funded = amount;
        }
        j.amount = funded;
        j.state = JobState.Funded;
        emit Funded(jobId, j.buyer, funded);
    }

    // ---------------------------------------------------------------------
    // assign
    // ---------------------------------------------------------------------

    /// @notice Assign a provider to a Funded Job.
    function assign(uint256 jobId, address provider) external {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        require(j.state == JobState.Funded, "PrismSettle: bad state");
        require(msg.sender == j.buyer, "PrismSettle: not buyer");
        require(provider != address(0), "PrismSettle: zero provider");
        j.provider = provider;
        j.state = JobState.Assigned;
        emit Assigned(jobId, provider);
    }

    // ---------------------------------------------------------------------
    // submit
    // ---------------------------------------------------------------------

    /// @notice Provider submits work. If a Hook is mounted, triggers
    ///         Hook.onSubmitted so the buyer may subsequently dispute.
    function submit(uint256 jobId, bytes32 deliverableHash, bytes32 proofHash) external {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        require(j.state == JobState.Assigned, "PrismSettle: bad state");
        require(msg.sender == j.provider, "PrismSettle: not provider");
        require(deliverableHash != bytes32(0), "PrismSettle: empty deliverable");
        require(proofHash != bytes32(0), "PrismSettle: empty proof");

        j.deliverableHash = deliverableHash;
        j.proofHash = proofHash;
        j.state = JobState.Submitted;
        emit Submitted(jobId, deliverableHash, proofHash);

        if (j.hook != address(0)) {
            ArbitrationHook(j.hook).onSubmitted(jobId);
        }
    }

    // ---------------------------------------------------------------------
    // complete — Evaluator only
    // ---------------------------------------------------------------------

    /// @notice Mark a Submitted Job as Completed and pay the provider.
    /// @dev    Only COMMERCE_EVALUATOR_ROLE may call.
    function complete(uint256 jobId) external onlyRole(COMMERCE_EVALUATOR_ROLE) {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        require(j.state == JobState.Submitted, "PrismSettle: bad state");
        j.state = JobState.Completed;
        require(paymentToken.transfer(j.provider, j.amount), "PrismSettle: payout failed");
        emit Completed(jobId, j.provider, j.amount);
    }

    // ---------------------------------------------------------------------
    // claimRefund
    // ---------------------------------------------------------------------

    /// @notice Buyer claims a refund after the deadline, or immediately
    ///         when an arbitration ruling=1 (buyer wins) is on record.
    function claimRefund(uint256 jobId) external {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        require(
            j.state == JobState.Funded || j.state == JobState.Assigned || j.state == JobState.Submitted,
            "PrismSettle: bad state"
        );
        require(msg.sender == j.buyer, "PrismSettle: not buyer");

        // Arbitration ruling=1 lifts the deadline requirement.
        if (j.hook != address(0)) {
            (,, uint8 ruling) = ArbitrationHook(j.hook).getHookState(jobId);
            if (ruling != 1) {
                require(block.timestamp >= j.deadline, "PrismSettle: deadline not reached");
            }
        } else {
            require(block.timestamp >= j.deadline, "PrismSettle: deadline not reached");
        }

        j.state = JobState.Refunded;
        require(paymentToken.transfer(j.buyer, j.amount), "PrismSettle: refund failed");
        emit Refunded(jobId, j.buyer, j.amount);
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
            address hook
        )
    {
        Job storage j = shardJobs[uint8(jobId & 0xFF)][jobId];
        return (j.state, j.buyer, j.provider, j.amount, j.deliverableHash, j.proofHash, j.deadline, j.hook);
    }

    /// @notice Buyer-only view used by ArbitrationHook.dispute to verify
    ///         the caller. Kept separate from getJobState so the Hook
    ///         does not need to decode the full tuple.
    function getJobBuyer(uint256 jobId) external view returns (address) {
        return shardJobs[uint8(jobId & 0xFF)][jobId].buyer;
    }
}
