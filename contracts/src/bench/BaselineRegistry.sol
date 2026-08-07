// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

/// @title BaselineRegistry
/// @notice V0 对照合约：与 PrismSettleRegistry 接口完全一致，但使用
///         单槽存储（无 256 分片）。用于压测对比 OCC abort rate。
/// @dev    与 V1 的唯一差异：validations[agentId] 替代 shardValidations[shard][agentId]。
///         其余功能（register/stake/submit/aggregate/seed/slash/EMA/decay）完全复刻 V1。
contract BaselineRegistry is AccessControl {
    // ---------------------------------------------------------------------
    // Constants（与 V1 一致）
    // ---------------------------------------------------------------------

    uint256 public constant EPOCH = 1 minutes;
    uint256 public constant UNSTAKE_LOCK = 7 days;
    uint256 public constant MAX_RECORDS = 100;
    uint256 public constant MAX_VALIDATIONS_PER_EPOCH = 50;
    uint256 public constant MIN_STAKE = 100 ether;
    uint256 public constant INACTIVE_DAYS = 30 days;
    uint256 public constant DECAY_PER_DAY = 0.01e18;
    uint96 public constant DEFAULT_SEED_SCORE = 0.7e18;

    bytes32 public constant REGISTRY_EVALUATOR_ROLE = keccak256("REGISTRY_EVALUATOR_ROLE");

    // ---------------------------------------------------------------------
    // Types（与 V1 一致）
    // ---------------------------------------------------------------------

    struct ValidationRecord {
        address validator;
        uint96 score;
        bytes32 proofHash;
        uint64 timestamp;
        uint8 source;
        uint256 jobId;
    }

    struct AgentMetadata {
        bool registered;
        address owner;
        string endpointUrl;
        string capabilities;
        uint64 lastAggregate;
        uint96 seedScore;
        uint64 registeredAt;
        uint64 taskCount;
        uint64 lastActivity;
    }

    struct StakeInfo {
        uint256 amount;
        uint256 pendingUnstake;
        uint64 unstakeAt;
    }

    // ---------------------------------------------------------------------
    // Storage — V0 单槽（与 V1 的唯一差异）
    // ---------------------------------------------------------------------

    /// @dev V0：单槽存储。所有 agent 的 validation 记录都写入同一个 mapping，
    ///      无分片隔离。这是 V0 与 V1 的核心差异，用于压测 OCC abort rate 对照。
    mapping(uint256 => ValidationRecord[]) public validations;

    mapping(uint256 => AgentMetadata) public agents;
    mapping(uint256 => uint256) public aggregatedScore;
    mapping(address => StakeInfo) public validatorStake;

    // ---------------------------------------------------------------------
    // Events（与 V1 一致）
    // ---------------------------------------------------------------------

    event AgentRegistered(uint256 indexed agentId, address indexed owner, string metadata);
    event ValidationSubmitted(
        uint256 indexed agentId,
        uint8 indexed shard, // V0 中 shard 恒为 0，保留字段以对齐 V1 事件签名
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

    function registerAgent(uint256 agentId, string calldata metadata) external {
        require(!agents[agentId].registered, "PrismSettle: already registered");
        agents[agentId] = AgentMetadata({
            registered: true,
            owner: msg.sender,
            endpointUrl: metadata,
            capabilities: "",
            lastAggregate: 0,
            seedScore: 0,
            registeredAt: uint64(block.timestamp),
            taskCount: 0,
            lastActivity: 0
        });
        emit AgentRegistered(agentId, msg.sender, metadata);
    }

    function seedAgent(uint256 agentId, uint96 seedScore) external onlyRole(DEFAULT_ADMIN_ROLE) {
        AgentMetadata storage info = agents[agentId];
        require(info.registered, "PrismSettle: agent not registered");
        require(info.seedScore == 0, "PrismSettle: already seeded");
        info.seedScore = seedScore;
        aggregatedScore[agentId] = seedScore;
    }

    // ---------------------------------------------------------------------
    // Validator staking
    // ---------------------------------------------------------------------

    function stake() external payable {
        require(msg.value > 0, "PrismSettle: zero stake");
        validatorStake[msg.sender].amount += msg.value;
        emit Staked(msg.sender, msg.value);
    }

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
    // High-frequency path — V0 单槽存储（无分片）
    // ---------------------------------------------------------------------

    /// @dev V0 与 V1 的核心差异：写入 validations[agentId] 而非 shardValidations[shard][agentId]。
    ///      这导致不同 agent 的并发写入可能命中相邻存储槽，触发 OCC 冲突。
    function submitValidation(uint256 agentId, uint96 score, bytes32 proofHash, uint256 jobId, uint8 source) external {
        AgentMetadata storage info = agents[agentId];
        require(info.registered, "PrismSettle: agent not registered");
        require(score <= 1e18, "PrismSettle: score > 1e18");

        if (source == 0) {
            require(validatorStake[msg.sender].amount >= MIN_STAKE, "PrismSettle: insufficient stake");
            require(jobId == 0, "PrismSettle: validator jobId must be 0");
        } else if (source == 1 || source == 2) {
            require(hasRole(REGISTRY_EVALUATOR_ROLE, msg.sender), "PrismSettle: not evaluator");
        } else {
            revert("PrismSettle: invalid source");
        }

        // V0：单槽存储，无分片路由
        ValidationRecord[] storage records = validations[agentId];

        if (source == 0) {
            require(records.length < MAX_VALIDATIONS_PER_EPOCH, "PrismSettle: epoch quota exceeded");
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

        info.lastActivity = uint64(block.timestamp);

        // V0 中 shard 恒为 0（保留字段以对齐 V1 事件签名，便于 Parser 复用）
        emit ValidationSubmitted(agentId, 0, msg.sender, score, proofHash, uint64(block.timestamp), source, jobId);
    }

    // ---------------------------------------------------------------------
    // Low-frequency path: aggregate（与 V1 一致，仅存储路径不同）
    // ---------------------------------------------------------------------

    function aggregateEpoch(uint256 agentId) external {
        AgentMetadata storage info = agents[agentId];
        require(info.registered, "PrismSettle: agent not registered");
        require(block.timestamp >= uint256(info.lastAggregate) + EPOCH, "PrismSettle: epoch not due");

        ValidationRecord[] storage records = validations[agentId];
        uint256 oldScore = aggregatedScore[agentId];

        if (records.length == 0) {
            info.lastAggregate = uint64(block.timestamp);
            (uint256 decayedScore, uint256 decay) = applyDecay(oldScore, info.lastActivity, block.timestamp);
            if (decayedScore != oldScore) {
                aggregatedScore[agentId] = decayedScore;
            }
            emit Aggregated(agentId, oldScore, decayedScore, 0, info.taskCount, decay);
            return;
        }

        uint256 processCount = records.length > MAX_RECORDS ? MAX_RECORDS : records.length;
        uint256 weightedSum = 0;
        uint256 totalWeight = 0;
        for (uint256 i = 0; i < processCount; i++) {
            ValidationRecord storage r = records[0];
            uint256 w = (r.source == 0) ? validatorStake[r.validator].amount : 1;
            if (w == 0) w = 1;
            weightedSum += uint256(r.score) * w;
            totalWeight += w;
            records[0] = records[records.length - 1];
            records.pop();
        }
        uint256 weighted = totalWeight == 0 ? info.seedScore : weightedSum / totalWeight;

        uint256 taskCountScaled = 1e18 + uint256(info.taskCount);
        uint256 newScore;
        if (weighted >= oldScore) {
            newScore = oldScore + (weighted - oldScore) * 1e18 / taskCountScaled;
        } else {
            newScore = oldScore - (oldScore - weighted) * 1e18 / taskCountScaled;
        }

        uint256 decayApplied;
        (newScore, decayApplied) = applyDecay(newScore, info.lastActivity, block.timestamp);
        if (newScore > 1e18) newScore = 1e18;

        aggregatedScore[agentId] = newScore;
        info.lastAggregate = uint64(block.timestamp);
        info.taskCount += uint64(processCount);
        emit Aggregated(agentId, oldScore, newScore, processCount, info.taskCount, decayApplied);
    }

    // ---------------------------------------------------------------------
    // Slashing
    // ---------------------------------------------------------------------

    function slash(address validator, bytes32 evidenceHash) external onlyRole(DEFAULT_ADMIN_ROLE) {
        uint256 amount = validatorStake[validator].amount;
        require(amount > 0, "PrismSettle: nothing to slash");
        validatorStake[validator].amount = 0;
        (bool ok,) = payable(address(0xdead)).call{value: amount}("");
        require(ok, "PrismSettle: burn failed");
        emit Slashed(validator, amount, evidenceHash);
    }

    // ---------------------------------------------------------------------
    // Rewards stub
    // ---------------------------------------------------------------------

    function claimRewards() external pure {
        revert("PrismSettle: V2 only");
    }

    // ---------------------------------------------------------------------
    // Views
    // ---------------------------------------------------------------------

    function getScore(uint256 agentId) external view returns (uint256) {
        uint256 score = aggregatedScore[agentId];
        uint64 lastActivity = agents[agentId].lastActivity;
        if (block.timestamp >= uint256(lastActivity) + INACTIVE_DAYS) {
            (uint256 decayed,) = applyDecay(score, lastActivity, block.timestamp);
            return decayed;
        }
        return score;
    }

    function getValidationCount(uint256 agentId) external view returns (uint256) {
        return validations[agentId].length;
    }

    function getValidation(uint256 agentId, uint256 index)
        external
        view
        returns (address validator, uint96 score, bytes32 proofHash, uint64 timestamp, uint8 source, uint256 jobId)
    {
        ValidationRecord storage r = validations[agentId][index];
        return (r.validator, r.score, r.proofHash, r.timestamp, r.source, r.jobId);
    }

    // ---------------------------------------------------------------------
    // Internal: inactivity decay（与 V1 一致）
    // ---------------------------------------------------------------------

    function applyDecay(uint256 score, uint64 lastActivity, uint256 currentTimestamp)
        internal
        pure
        returns (uint256 decayed, uint256 decay)
    {
        if (lastActivity == 0 || currentTimestamp < uint256(lastActivity) + INACTIVE_DAYS) {
            return (score, 0);
        }
        uint256 inactiveDays = (currentTimestamp - uint256(lastActivity)) / 1 days;
        uint256 totalDecay = inactiveDays * DECAY_PER_DAY;
        if (totalDecay >= score) {
            return (0, score);
        }
        return (score - totalDecay, totalDecay);
    }
}
