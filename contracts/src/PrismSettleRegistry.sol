// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

/// @title PrismSettleRegistry
/// @notice ERC-8004 Validation Registry with sharded storage layout for
///         Monad's optimistic concurrent execution (OCC).
///
/// @dev Design overview
///      High-frequency path (submitValidation) only appends to a per-shard
///      array keyed by (shard, agentId). Because each agent maps to exactly
///      one shard via the low 8 bits of agentId, two transactions touching
///      different agents almost always touch disjoint storage slots. This
///      keeps the OCC read/write sets from overlapping and lets Monad
///      parallelize the transactions without aborting.
///
///      The shared write target (aggregatedScore[agentId]) is updated only
///      by the low-frequency aggregateEpoch path, which is rate-limited per
///      agent (EPOCH) and intended to be called by a keeper bot that
///      staggers calls across agents to avoid collisions.
contract PrismSettleRegistry is AccessControl {
    // ---------------------------------------------------------------------
    // Constants
    // ---------------------------------------------------------------------

    /// @dev Number of shards. 8 bits -> 256 slots; 500 concurrent txs yield
    ///      ~30% raw collision probability, dropping below 5% with delayed
    ///      aggregation.
    uint256 public constant SHARD_COUNT = 256;

    /// @dev Minimum stake a validator must lock before submitting validations.
    uint256 public constant MIN_STAKE = 5 ether;

    /// @dev Maximum stake per validator (anti-centralization). Prevents a
    ///      single validator from accumulating disproportionate influence over
    ///      reputation scores via stake-weighted aggregation.
    uint256 public constant MAX_STAKE = 200 ether;

    /// @dev Minimum interval between two aggregateEpoch calls for the same
    ///      agent. Staggers aggregation across agents so the slow path does
    ///      not become a write-contention hotspot.
    uint256 public constant EPOCH = 1 minutes;

    /// @dev unstake lock-up period.
    uint256 public constant UNSTAKE_LOCK = 7 days;

    /// @dev Max records processed per aggregateEpoch call (gas protection).
    uint256 public constant MAX_RECORDS = 100;

    /// @dev Per-agent per-epoch Validator validation cap (anti-sybil).
    uint256 public constant MAX_VALIDATIONS_PER_EPOCH = 50;

    /// @dev Per-agent per-epoch Evaluator validation cap (anti-sybil).
    ///      Evaluator score submissions are also bounded to prevent reputation
    ///      manipulation. Source=2 (arbitration) is exempt because it is a
    ///      penalty path, not a score-boosting path.
    uint256 public constant MAX_EVALUATOR_RECORDS_PER_EPOCH = 20;

    /// @dev Inactivity decay window: scores start decaying after this many
    ///      days of inactivity.
    uint256 public constant INACTIVE_DAYS = 30;

    /// @dev Inactivity decay rate: 0.01e18 per day past INACTIVE_DAYS.
    uint256 public constant DECAY_PER_DAY = 0.01e18;

    /// @dev Seed Phase preset initial score for official Agents.
    uint96 public constant DEFAULT_SEED_SCORE = 0.7e18;

    // ---------------------------------------------------------------------
    // Roles
    // ---------------------------------------------------------------------

    /// @dev Evaluator holds this role to call submitValidation with
    ///      source in {1,2} without stake.
    bytes32 public constant REGISTRY_EVALUATOR_ROLE = keccak256("REGISTRY_EVALUATOR_ROLE");

    // ---------------------------------------------------------------------
    // Types
    // ---------------------------------------------------------------------

    struct ValidationRecord {
        address validator; // Validator address or Evaluator address (source != 0)
        uint96 score; // 0..1e18 fixed-point (1e18 = 1.0)
        bytes32 proofHash; // deliverable proof_hash
        uint64 timestamp;
        uint8 source; // 0=Validator / 1=Evaluator-Job / 2=Evaluator-Arbitration / 3=Buyer
        uint256 jobId; // 0 when source=0; Job id when source in {1,2,3}
    }

    struct AgentMetadata {
        bool registered;
        address owner;
        string metadata; // JSON: {"endpointUrl":"...","capabilities":"...","name":"...","description":"..."}
        string capabilities; // JSON string, capability tags (deprecated, use metadata field)
        uint64 lastAggregate;
        uint96 seedScore; // Seed Phase preset initial score 0.7e18
        uint64 registeredAt;
        uint64 taskCount; // aggregated task count (EMA smoothing alpha=1/(1+taskCount))
        uint64 lastActivity; // last submitValidation timestamp (inactivity decay)
    }

    struct StakeInfo {
        uint256 amount; // active stake (can submit validations)
        uint256 pendingUnstake; // locked amount awaiting withdrawUnstaked
        uint64 unstakeAt; // 0 = no pending unstake; >0 = in 7-day unlock period
    }

    // ---------------------------------------------------------------------
    // Storage
    // ---------------------------------------------------------------------

    // High-frequency write path: sharded by low 8 bits of agentId.
    mapping(uint8 => mapping(uint256 => ValidationRecord[])) public shardValidations;

    // Trusted Job contracts allowed to call submitValidation with source=3 (Buyer).
    mapping(address => bool) public trustedJobs;

    // Low-frequency write path: only touched by aggregateEpoch.
    mapping(uint256 => uint256) public aggregatedScore;

    // Per-agent metadata (effectively read-only during the hot path).
    mapping(uint256 => AgentMetadata) public agents;

    // Validator stake. Stake-weighted aggregation lives off the hot path.
    mapping(address => StakeInfo) public validatorStake;

    // ---------------------------------------------------------------------
    // Events
    // ---------------------------------------------------------------------

    event AgentRegistered(uint256 indexed agentId, address indexed owner, string metadata);
    event ValidationSubmitted(
        uint256 indexed agentId,
        uint8 indexed shard,
        address indexed validator,
        uint96 score,
        bytes32 proofHash,
        uint64 timestamp,
        uint8 source,
        uint256 jobId
    );
    event Aggregated(
        uint256 indexed agentId, uint256 oldScore, uint256 newScore, uint256 count, uint64 taskCount, uint256 decay
    );
    event Staked(address indexed validator, uint256 amount);
    event Slashed(address indexed validator, uint256 amount, bytes32 evidenceHash);
    event UnstakeStarted(address indexed validator, uint256 amount, uint64 unlockAt);
    event UnstakeWithdrawn(address indexed validator, uint256 amount);

    // ---------------------------------------------------------------------
    // Constructor
    // ---------------------------------------------------------------------

    constructor() {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
    }

    // ---------------------------------------------------------------------
    // Agent registration
    // ---------------------------------------------------------------------

    /// @notice Register an agent. Anyone may register an agentId; ownership
    ///         is set to the caller and is used for slashing authorization.
    /// @param metadata JSON string containing endpointUrl / capabilities.
    ///                  Format: {"endpointUrl":"...","capabilities":"..."}
    function registerAgent(uint256 agentId, string calldata metadata) external {
        require(!agents[agentId].registered, "PrismSettle: already registered");
        agents[agentId] = AgentMetadata({
            registered: true,
            owner: msg.sender,
            metadata: metadata,
            capabilities: "",
            lastAggregate: 0,
            seedScore: 0,
            registeredAt: uint64(block.timestamp),
            taskCount: 0,
            lastActivity: 0
        });
        emit AgentRegistered(agentId, msg.sender, metadata);
    }

    /// @notice Seed Phase: set an official Agent's initial score.
    /// @dev    Only deployer may call, one-shot per agent. Sets seedScore
    ///         and aggregatedScore so the agent is immediately usable.
    ///         Called by Deploy.s.sol / scripts/seed.sh for 4 official agents.
    function seedAgent(uint256 agentId, uint96 seedScore) external onlyRole(DEFAULT_ADMIN_ROLE) {
        AgentMetadata storage info = agents[agentId];
        require(info.registered, "PrismSettle: agent not registered");
        require(info.seedScore == 0, "PrismSettle: already seeded");
        info.seedScore = seedScore;
        aggregatedScore[agentId] = seedScore;
    }

    /// @notice Owner address of an agent. grabJob uses this to enforce that
    ///         only the agent's own wallet (its operator) may claim jobs —
    ///         agents stay autonomous; no third-party delegation.
    function agentOwner(uint256 agentId) external view returns (address) {
        return agents[agentId].owner;
    }

    /// @notice Mark or unmark a contract as a trusted Job contract. Trusted
    ///         Job contracts may call submitValidation with source=3 (Buyer).
    /// @param job      The Job contract address.
    /// @param trusted  Whether the contract is trusted.
    function setTrustedJob(address job, bool trusted) external onlyRole(DEFAULT_ADMIN_ROLE) {
        trustedJobs[job] = trusted;
    }

    // ---------------------------------------------------------------------
    // Validator staking
    // ---------------------------------------------------------------------

    /// @notice Lock stake. Must reach MIN_STAKE before submitting validations.
    function stake() external payable {
        require(msg.value > 0, "PrismSettle: zero stake");
        uint256 newTotal = validatorStake[msg.sender].amount + msg.value;
        require(newTotal <= MAX_STAKE, "PrismSettle: max stake exceeded");
        validatorStake[msg.sender].amount = newTotal;
        emit Staked(msg.sender, msg.value);
    }

    /// @notice Begin unstake. Funds enter 7-day unlock period.
    /// @dev    V1 simplified: only one in-flight unstake at a time.
    ///         For multiple partial unstakes, must first withdrawUnstaked
    ///         to reset unstakeAt to 0 before initiating a new unstake.
    function unstake(uint256 amount) external {
        StakeInfo storage s = validatorStake[msg.sender];
        require(s.amount >= amount, "PrismSettle: insufficient stake");
        require(s.unstakeAt == 0, "PrismSettle: already unstaking");
        require(amount > 0, "PrismSettle: zero unstake");
        s.amount -= amount;
        s.pendingUnstake = amount;
        s.unstakeAt = uint64(block.timestamp);
        emit UnstakeStarted(msg.sender, amount, s.unstakeAt + uint64(UNSTAKE_LOCK));
    }

    /// @notice Withdraw unstaked funds after the 7-day lock expires.
    /// @dev    Pays out the pendingUnstake amount locked in unstake().
    ///         active stake (s.amount) is untouched.
    function withdrawUnstaked() external {
        StakeInfo storage s = validatorStake[msg.sender];
        require(s.unstakeAt > 0, "PrismSettle: no pending unstake");
        require(block.timestamp >= uint256(s.unstakeAt) + UNSTAKE_LOCK, "PrismSettle: lock not expired");
        uint256 payout = s.pendingUnstake;
        s.pendingUnstake = 0;
        s.unstakeAt = 0;
        (bool ok,) = payable(msg.sender).call{value: payout}("");
        require(ok, "PrismSettle: transfer failed");
        emit UnstakeWithdrawn(msg.sender, payout);
    }

    // ---------------------------------------------------------------------
    // High-frequency path: submit a validation
    // ---------------------------------------------------------------------

    /// @notice Append a validation record for an agent.
    /// @dev    This is the hot path. It writes ONLY shardValidations[shard][agentId]
    ///         and agents[agentId].lastActivity. aggregatedScore is intentionally
    ///         NOT touched here so that concurrent submits for the same agent
    ///         do not all collide on the same slot. Aggregation is deferred
    ///         to aggregateEpoch.
    /// @param agentId   Target agent.
    /// @param score     0..1e18 fixed-point.
    /// @param proofHash Hash of off-chain proof (e.g. signature bundle).
    /// @param jobId     0 for Validator channel; Job id for Evaluator/Buyer channels.
    /// @param source    0=Validator / 1=Evaluator-Job / 2=Evaluator-Arbitration / 3=Buyer.
    function submitValidation(uint256 agentId, uint96 score, bytes32 proofHash, uint256 jobId, uint8 source) external {
        AgentMetadata storage info = agents[agentId];
        require(info.registered, "PrismSettle: agent not registered");
        require(score <= 1e18, "PrismSettle: score > 1e18");

        if (source == 0) {
            // Validator channel
            require(validatorStake[msg.sender].amount >= MIN_STAKE, "PrismSettle: insufficient stake");
            require(jobId == 0, "PrismSettle: validator jobId must be 0");
        } else if (source == 1 || source == 2) {
            // Evaluator channel
            require(hasRole(REGISTRY_EVALUATOR_ROLE, msg.sender), "PrismSettle: not evaluator");
        } else if (source == 3) {
            // Buyer channel — called from trusted Job contract
            require(trustedJobs[msg.sender], "PrismSettle: not trusted job");
        } else {
            revert("PrismSettle: invalid source");
        }

        uint8 shard = shardOf(agentId);
        ValidationRecord[] storage records = shardValidations[shard][agentId];

        // Anti-sybil: per-epoch caps by source type.
        // - source=0 (Validator): MAX_VALIDATIONS_PER_EPOCH, stake-gated.
        // - source=1 (Evaluator main): MAX_EVALUATOR_RECORDS_PER_EPOCH.
        // - source=2 (Arbitration penalty): exempt (penalty path, not score-boosting).
        // - source=3 (Buyer): exempt (one-time rating per job, not score-boosting).
        if (source == 0) {
            uint256 count;
            for (uint256 i = 0; i < records.length; i++) {
                if (records[i].source == 0) {
                    unchecked { ++count; }
                }
            }
            require(count < MAX_VALIDATIONS_PER_EPOCH, "PrismSettle: validator epoch quota exceeded");
        } else if (source == 1) {
            uint256 count;
            for (uint256 i = 0; i < records.length; i++) {
                if (records[i].source == 1) {
                    unchecked { ++count; }
                }
            }
            require(count < MAX_EVALUATOR_RECORDS_PER_EPOCH, "PrismSettle: evaluator epoch quota exceeded");
        }

        records.push(
            ValidationRecord({
                validator: msg.sender,
                score: score,
                proofHash: proofHash,
                timestamp: uint64(block.timestamp),
                source: source,
                jobId: jobId
            })
        );

        // Update lastActivity, reset inactivity decay timer.
        info.lastActivity = uint64(block.timestamp);

        emit ValidationSubmitted(agentId, shard, msg.sender, score, proofHash, uint64(block.timestamp), source, jobId);
    }

    // ---------------------------------------------------------------------
    // Offchain-computed score setter
    // ---------------------------------------------------------------------

    /// @dev EMA smoothing factor: 0.3 (fixed-point 1e18).
    uint256 public constant ALPHA = 0.3e18;

    /// @dev Dual-factor completion bonus: +0.005e18 per completed job
    ///      (source=3 Buyer rating record) per epoch.
    uint256 public constant COMPLETION_BONUS = 0.005e18;

    /// @dev Dual-factor completion bonus cap per epoch (≈10 jobs/epoch).
    uint256 public constant COMPLETION_BONUS_EPOCH_CAP = 0.05e18;

    /// @dev Dual-factor rating adjustment coefficient: (score − 0.5e18) × 0.05.
    uint256 public constant RATING_ADJUST_COEF = 0.05e18;

    /// @dev Dual-factor rating adjustment cap per epoch (±0.02e18).
    uint256 public constant RATING_ADJUST_EPOCH_CAP = 0.02e18;

    /// @notice Set the aggregated score for an agent. Called by the offchain
    ///         Keeper after computing the weighted average, EMA smoothing,
    ///         arbitration penalty, and inactivity decay off-chain.
    ///         Replaces the on-chain aggregateEpoch for the primary path.
    /// @param agentId   The target agent.
    /// @param newScore  The computed score (0..1e18).
    function setAggregatedScore(uint256 agentId, uint256 newScore) external onlyRole(REGISTRY_EVALUATOR_ROLE) {
        AgentMetadata storage info = agents[agentId];
        require(info.registered, "PrismSettle: agent not registered");
        require(newScore <= 1e18, "PrismSettle: score > 1e18");

        uint256 oldScore = aggregatedScore[agentId];
        aggregatedScore[agentId] = newScore;
        info.lastAggregate = uint64(block.timestamp);

        emit Aggregated(agentId, oldScore, newScore, 0, info.taskCount, 0);
    }

    // ---------------------------------------------------------------------
    // Low-frequency path: aggregate an agent's shard
    // ---------------------------------------------------------------------

    /// @notice Aggregate the sharded validation records into a single score.
    /// @dev    Rate-limited per agent to EPOCH so a keeper bot can stagger
    ///         these calls across agents and avoid re-creating the shared-slot
    ///         hotspot on the slow path.
    function aggregateEpoch(uint256 agentId) external {
        AgentMetadata storage info = agents[agentId];
        require(info.registered, "PrismSettle: agent not registered");
        require(block.timestamp >= uint256(info.lastAggregate) + EPOCH, "PrismSettle: epoch not due");

        uint8 shard = shardOf(agentId);
        ValidationRecord[] storage records = shardValidations[shard][agentId];
        uint256 oldScore = aggregatedScore[agentId];

        if (records.length == 0) {
            // No new validation: still check inactivity decay.
            info.lastAggregate = uint64(block.timestamp);
            (uint256 decayedScore, uint256 decay) = applyDecay(oldScore, info.lastActivity, block.timestamp);
            if (decayedScore != oldScore) {
                aggregatedScore[agentId] = decayedScore;
            }
            emit Aggregated(agentId, oldScore, decayedScore, 0, info.taskCount, decay);
            return;
        }

        // 1. Stake-weighted average: Validator weighted by stake; Evaluator weight = 1.
        //    source=3 (Buyer rating) records are EXCLUDED from the weighted
        //    average — they feed the dual-factor channel in step 2b.
        // Gas protection + FIFO: process at most MAX_RECORDS oldest records per call,
        // newer ones deferred to next epoch. O(1) head removal: tail overwrites head
        // then pop (order shuffled but weighted average unaffected).
        uint256 processCount = records.length > MAX_RECORDS ? MAX_RECORDS : records.length;
        uint256 weightedSum = 0;
        uint256 totalWeight = 0;
        uint256 buyerCount = 0;
        uint256 buyerSum = 0;
        for (uint256 i = 0; i < processCount; i++) {
            ValidationRecord storage r = records[0];
            if (r.source == 3) {
                // Buyer satisfaction record: dual-factor channel.
                buyerCount++;
                buyerSum += uint256(r.score);
                records[0] = records[records.length - 1];
                records.pop();
                continue;
            }
            uint256 w = (r.source == 0) ? validatorStake[r.validator].amount : 1;
            if (w == 0) w = 1; // fallback: slashed Validator's historical records still participate
            weightedSum += uint256(r.score) * w;
            totalWeight += w;
            records[0] = records[records.length - 1];
            records.pop();
        }
        uint256 weighted = totalWeight == 0 ? info.seedScore : weightedSum / totalWeight;

        // 2. EMA smoothing (source=0/1/2 only): newScore = oldScore + ALPHA * (weighted - oldScore)
        //    ALPHA = 0.3 (fixed). Fixed alpha ensures agents can always
        //    recover from a bad rating, unlike taskCount-based alpha which
        //    asymptotically approaches zero change.
        uint256 newScore = oldScore;
        if (totalWeight > 0) {
            if (weighted >= oldScore) {
                newScore = oldScore + (weighted - oldScore) * ALPHA / 1e18;
            } else {
                newScore = oldScore - (oldScore - weighted) * ALPHA / 1e18;
            }
        }

        // 2b. Dual-factor channel (source=3 Buyer ratings):
        //     completion bonus (+0.005e18/record, epoch-capped) plus a rating
        //     nudge ((avg − 0.5e18) × 0.05, epoch-capped ±0.02e18).
        if (buyerCount > 0) {
            uint256 bonus = COMPLETION_BONUS * buyerCount;
            if (bonus > COMPLETION_BONUS_EPOCH_CAP) bonus = COMPLETION_BONUS_EPOCH_CAP;
            uint256 avg = buyerSum / buyerCount;
            if (avg >= 0.5e18) {
                uint256 up = (avg - 0.5e18) * RATING_ADJUST_COEF / 1e18;
                if (up > RATING_ADJUST_EPOCH_CAP) up = RATING_ADJUST_EPOCH_CAP;
                newScore += bonus + up;
            } else {
                uint256 down = (0.5e18 - avg) * RATING_ADJUST_COEF / 1e18;
                if (down > RATING_ADJUST_EPOCH_CAP) down = RATING_ADJUST_EPOCH_CAP;
                newScore = newScore + bonus > down ? newScore + bonus - down : 0;
            }
        }

        // 3. Inactivity decay: after INACTIVE_DAYS of inactivity, deduct DECAY_PER_DAY per day.
        uint256 decayApplied;
        (newScore, decayApplied) = applyDecay(newScore, info.lastActivity, block.timestamp);

        // 4. Boundary protection
        if (newScore > 1e18) newScore = 1e18;

        aggregatedScore[agentId] = newScore;
        info.lastAggregate = uint64(block.timestamp);
        info.taskCount += uint64(processCount);
        // NOTE: lastActivity is updated only in submitValidation (real business scoring),
        // never here. aggregateEpoch triggered by Keeper does not indicate Agent activity.

        emit Aggregated(agentId, oldScore, newScore, processCount, info.taskCount, decayApplied);
    }

    // ---------------------------------------------------------------------
    // Slashing (V1: Owner only)
    // ---------------------------------------------------------------------

    /// @notice Slash a validator's stake. V1: only deployer may call.
    /// @dev    V1 simplified: burns 100% of stake. Formula-based penalty
    ///         is computed off-chain by Evaluator and submitted
    ///         via submitValidation(source=2); the contract does not
    ///         enforce a specific slash formula.
    function slash(address validator, bytes32 evidenceHash) external onlyRole(DEFAULT_ADMIN_ROLE) {
        uint256 amount = validatorStake[validator].amount;
        require(amount > 0, "PrismSettle: nothing to slash");
        validatorStake[validator].amount = 0;
        // Burn by sending to a known dead address.
        (bool ok,) = payable(address(0xdead)).call{value: amount}("");
        require(ok, "PrismSettle: burn failed");
        emit Slashed(validator, amount, evidenceHash);
    }

    // ---------------------------------------------------------------------
    // Views
    // ---------------------------------------------------------------------

    /// @dev Shard of an agent. Low 8 bits of agentId.
    function shardOf(uint256 agentId) public pure returns (uint8) {
        return uint8(agentId & 0xFF);
    }

    /// @notice Get an agent's current score, applying inactivity decay at
    ///         read time. No storage write, zero gas, real-time score.
    function getScore(uint256 agentId) external view returns (uint256) {
        uint256 stored = aggregatedScore[agentId];
        (uint256 decayed,) = applyDecay(stored, agents[agentId].lastActivity, block.timestamp);
        return decayed;
    }

    function getValidationCount(uint256 agentId) external view returns (uint256) {
        return shardValidations[shardOf(agentId)][agentId].length;
    }

    function getValidation(uint256 agentId, uint256 index)
        external
        view
        returns (address validator, uint96 score, bytes32 proofHash, uint64 timestamp, uint8 source, uint256 jobId)
    {
        ValidationRecord storage r = shardValidations[shardOf(agentId)][agentId][index];
        return (r.validator, r.score, r.proofHash, r.timestamp, r.source, r.jobId);
    }

    // ---------------------------------------------------------------------
    // Rewards (V1 stub)
    // ---------------------------------------------------------------------

    /// @notice V1 stub. Reverts; rewards claimed via V2 mechanism.
    function claimRewards() external pure {
        revert("PrismSettle: V2 only");
    }

    // ---------------------------------------------------------------------
    // Internal: inactivity decay
    // ---------------------------------------------------------------------

    /// @dev Inactivity decay: deduct DECAY_PER_DAY per day after INACTIVE_DAYS.
    ///      Pure function, does not write storage; caller is responsible for
    ///      persistence. Returns (newScore, decay).
    function applyDecay(uint256 score, uint256 lastActivity, uint256 nowTs)
        internal
        pure
        returns (uint256 newScore, uint256 decay)
    {
        if (lastActivity == 0 || nowTs <= lastActivity) {
            return (score, 0);
        }
        uint256 idleDays = (nowTs - lastActivity) / 1 days;
        if (idleDays <= INACTIVE_DAYS) {
            return (score, 0);
        }
        uint256 decayDays = idleDays - INACTIVE_DAYS;
        decay = decayDays * DECAY_PER_DAY;
        newScore = score > decay ? score - decay : 0;
    }
}
