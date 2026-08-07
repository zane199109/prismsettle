-- ============================================================================
-- PrismSettle Database Schema (PostgreSQL)
-- ============================================================================
-- Core tables for the offchain indexer + evaluator + REST API.
-- GORM AutoMigrate creates these automatically; this file is provided for
-- manual setup, schema review, and documentation.
--
-- Generated: 2026-08-05
-- ============================================================================

-- ─── 1. Chain Events (generic event store) ──────────────────────────────────
-- Stores all emitted events across all monitored chains. This includes:
--   - ERC20 transfers       (ERC20_TRANSFER)
--   - PrismSettle registry  (PRISM_AGENT_REGISTERED, PRISM_AGGREGATED, etc.)
--   - PrismSettle Job       (PRISM_JOB_CREATED, PRISM_JOB_COMPLETED, etc.)
--   - ArbitrationHook       (PRISM_DISPUTED, PRISM_DISPUTE_RESOLVED, etc.)
--
-- Reputation history is derived from PRISM_AGGREGATED events filtered by
-- agentId (stored in the `to` field).

CREATE TABLE IF NOT EXISTS chain_events (
    id           BIGSERIAL       PRIMARY KEY,
    chain_name   VARCHAR(32)     NOT NULL,
    chain_type   VARCHAR(16)     NOT NULL,          -- 'evm'
    tx_type      VARCHAR(16),                       -- 'native' | 'token'
    event_type   VARCHAR(32)     NOT NULL,           -- e.g. 'PRISM_AGGREGATED'
    token_addr   VARCHAR(96),                        -- token address OR auxiliary hash (proofHash/deliverableHash)
    from_addr    VARCHAR(96),                        -- sender / operator
    to_addr      VARCHAR(96),                        -- receiver / agentId (hex)
    value        NUMERIC(78,0),                      -- amount / score
    symbol       VARCHAR(96),                        -- token symbol OR auxiliary field (source / buyer)
    decimals     SMALLINT        DEFAULT 18,
    tx_hash      VARCHAR(96)     NOT NULL,
    log_index    BIGINT          NOT NULL,
    block_number BIGINT          NOT NULL,
    block_time   BIGINT          NOT NULL,
    contract     VARCHAR(96),                        -- contract address
    status       SMALLINT        DEFAULT 1,          -- 1=Success, 0=Failed
    created_at   TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,

    -- Unique constraint for idempotent event insertion
    CONSTRAINT uq_chain_events_tx_log UNIQUE (tx_hash, log_index)
);

-- Indexes (production-critical)
CREATE INDEX IF NOT EXISTS idx_chain_event_chain_block
    ON chain_events (chain_name, block_number DESC);
CREATE INDEX IF NOT EXISTS idx_chain_event_contract_type
    ON chain_events (contract, event_type);
CREATE INDEX IF NOT EXISTS idx_chain_events_from_lower
    ON chain_events (LOWER(from_addr));
CREATE INDEX IF NOT EXISTS idx_chain_events_to_lower
    ON chain_events (LOWER(to_addr));
CREATE INDEX IF NOT EXISTS idx_chain_events_contract_lower
    ON chain_events (LOWER(contract));

-- ─── Reputation query accelerator ────────────────────────────────────────────
-- Optimizes the /reputation/history query:
--   WHERE chain_name = ?
--     AND event_type IN ('PRISM_AGGREGATED', 'PRISM_VALIDATION_SUBMITTED', 'PRISM_SLASHED')
--     AND LOWER(to_addr) = LOWER(?)
--   ORDER BY block_time DESC, block_number DESC
--   LIMIT ?
--
-- The composite index with functional LOWER(to_addr) covers all filter
-- conditions in one b-tree scan, avoiding a bitmap heap scan across the
-- entire event_type IN list for a given agent. The leading column
-- (chain_name) + function-based (LOWER(to_addr)) filter the partition
-- most aggressively; event_type as 3rd column narrows further; trailing
-- block_time DESC allows an index-only ORDER BY without a separate sort.
CREATE INDEX IF NOT EXISTS idx_chain_events_reputation
    ON chain_events (chain_name, LOWER(to_addr), event_type, block_time DESC, block_number DESC);

-- The same leading columns also accelerate GetPrismScore (latest score):
--   WHERE chain_name = ?
--     AND event_type = 'PRISM_AGGREGATED'
--     AND LOWER(to_addr) = LOWER(?)
--   ORDER BY block_number DESC, log_index DESC
--   LIMIT 1
-- PostgreSQL can use the first 3 columns of the same index and then scan
-- until it finds the first match (block_number DESC is a suffix of the index
-- sort order, so the first matching row is already the latest).

-- ─── 2. Chain Block State (sync checkpoint) ─────────────────────────────────
-- Tracks the latest processed block per chain + contract.
-- Used by the listener to resume from where it left off after restart.

CREATE TABLE IF NOT EXISTS chain_block_states (
    id            BIGSERIAL    PRIMARY KEY,
    chain_name    VARCHAR(32)  NOT NULL,
    contract_addr VARCHAR(96)  NOT NULL,
    last_block    BIGINT       NOT NULL,
    block_hash    VARCHAR(96),
    created_at    TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_chain_block_state UNIQUE (chain_name, contract_addr)
);

-- ─── 3. Agent Registry ──────────────────────────────────────────────────────
-- Mirrors AgentRegistered events from PrismSettleRegistry. Stores agent
-- identity, metadata (capabilities, name), and endpoint URL.
-- Used by the /agents endpoint to list available agents.
--
-- Field mapping:
--   agent_id   → hex uint256 (e.g. '0x3333')
--   owner      → operator wallet address (from event)
--   metadata   → JSON string: {"name":"...", "capabilities":["..."], "description":"..."}
--   endpoint   → extracted from metadata JSON if present

CREATE TABLE IF NOT EXISTS agent_registry (
    id            BIGSERIAL    PRIMARY KEY,
    chain_name    VARCHAR(32)  NOT NULL,
    agent_id      VARCHAR(96)  NOT NULL,        -- hex uint256
    owner         VARCHAR(96),                   -- operator wallet address
    metadata      TEXT,                          -- JSON: {name, capabilities, description}
    endpoint      VARCHAR(255),                  -- HTTP endpoint URL
    registered_at BIGINT       NOT NULL,         -- block_time of registration
    tx_hash       VARCHAR(96),
    block_number  BIGINT,
    created_at    TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_agent_registry UNIQUE (chain_name, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_registry_owner
    ON agent_registry (owner);

-- ─── 4. Trust Thresholds ────────────────────────────────────────────────────
-- Per-agent (or global default) ALLOW/DENY thresholds for the trust pre-check.
-- agent_id = '' represents the global default; per-agent rows override it.
--
-- Decision rule:
--   score >= allow_threshold  → ALLOW
--   score <  deny_threshold   → DENY
--   otherwise                 → REQUIRE_VALIDATION

CREATE TABLE IF NOT EXISTS trust_thresholds (
    id              BIGSERIAL       PRIMARY KEY,
    agent_id        VARCHAR(96)     NOT NULL DEFAULT '',   -- '' = global default
    allow_threshold NUMERIC(78,0)   NOT NULL DEFAULT 800000000000000000,  -- 0.8e18
    deny_threshold  NUMERIC(78,0)   NOT NULL DEFAULT 300000000000000000,  -- 0.3e18
    created_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_trust_threshold_agent UNIQUE (agent_id)
);

-- ─── 5. Decision Logs ───────────────────────────────────────────────────────
-- Records each Evaluator decision (validation result). The Evaluator writes
-- one row per job submission it processes. Used for audit trail and
-- debugging.

CREATE TABLE IF NOT EXISTS decision_logs (
    id              BIGSERIAL       PRIMARY KEY,
    job_id          VARCHAR(96)     NOT NULL,
    agent_id        VARCHAR(96)     NOT NULL,       -- provider agentId
    score           NUMERIC(78,0),                  -- evaluator score (0..1e18)
    reason          TEXT,                           -- evaluator reason
    decision        VARCHAR(32),                    -- 'approved' | 'rejected' | 'timeout'
    source          VARCHAR(16),                    -- 'evaluator' | 'validator'
    invalid         BOOLEAN         DEFAULT FALSE,  -- marked by reorg rollback
    tx_hash         VARCHAR(96),
    block_number    BIGINT,
    created_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_decision_log_job UNIQUE (job_id, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_decision_logs_agent
    ON decision_logs (agent_id);
CREATE INDEX IF NOT EXISTS idx_decision_logs_invalid
    ON decision_logs (invalid) WHERE invalid = FALSE;

-- ─── 6. Reorg Events (Phase 9) ──────────────────────────────────────────────
-- Records each chain reorg detected by the listener. Used by /perf/reorg-feed.

CREATE TABLE IF NOT EXISTS reorg_events (
    id              BIGSERIAL    PRIMARY KEY,
    chain_name      VARCHAR(32)  NOT NULL,
    contract_addr   VARCHAR(96)  NOT NULL,
    from_block      BIGINT       NOT NULL,       -- first orphaned block
    to_block        BIGINT       NOT NULL,        -- last orphaned block
    old_hash        VARCHAR(96),
    new_hash        VARCHAR(96),
    rollback_depth  INT          NOT NULL,
    rolled_back_rows BIGINT      DEFAULT 0,
    detected_at     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_reorg_event UNIQUE (chain_name, from_block, to_block)
);

CREATE INDEX IF NOT EXISTS idx_reorg_chain_detected
    ON reorg_events (chain_name, detected_at DESC);

-- ─── 7. Performance Results (Phase 9) ───────────────────────────────────────
-- Stores V0 vs V1 benchmark results. Used by /perf/v0-v1-comparison.

CREATE TABLE IF NOT EXISTS perf_results (
    id              BIGSERIAL    PRIMARY KEY,
    v0_abort_rate   DOUBLE PRECISION NOT NULL,
    v1_abort_rate   DOUBLE PRECISION NOT NULL,
    v0_throughput   DOUBLE PRECISION NOT NULL,
    v1_throughput   DOUBLE PRECISION NOT NULL,
    meets_fr_t06    BOOLEAN      NOT NULL DEFAULT FALSE,
    run_at          TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_perf_results_run_at
    ON perf_results (run_at DESC);

-- ============================================================================
-- Reputation History Query (reference)
-- ============================================================================
-- The reputation history for an agent is queried from chain_events:
--
-- SELECT
--     block_time,
--     value  AS score,
--     event_type
-- FROM chain_events
-- WHERE event_type IN ('PRISM_AGGREGATED', 'PRISM_VALIDATION_SUBMITTED', 'PRISM_SLASHED')
--   AND LOWER(to_addr) = LOWER(:agentId)
-- ORDER BY block_time DESC
-- LIMIT :limit;
--