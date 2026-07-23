# PrismSettle — Product Requirements Document

> **Version**: 1.0
> **Date**: 2026-06-29
> **Status**: Active development — Sprint 1 in progress
> **Author**: PrismSettle Team

---

## 1. Executive Summary

### 1.1 What is PrismSettle

PrismSettle is a **sharded validation registry** for the on-chain agent economy. It solves the reputation fragmentation problem: when 500+ AI agents transact concurrently on Monad, naive single-slot reputation storage causes ~60% transaction aborts due to Monad's Optimistic Concurrency Control (OCC). PrismSettle shards writes across 256 storage slots, dropping the abort rate to ~5%, and aggregates scores on a delayed, rate-limited path.

### 1.2 Positioning

| Goal | Priority | Implication |
|---|---|---|
| **Job-seeking portfolio** (infra role) | Primary | Code depth matters: reorg handling, sequential commit, tests, observability |
| **Hackathon award** | Secondary | Demo polish + narrative: agent-economy marketplace story on top of infra |

When tradeoffs arise, prefer infra depth over demo polish. The demo layer is built on top of solid infra, not instead of it.

### 1.3 Tagline

> *Sharded validation registry for the agent economy — 256 shards, ~5% abort rate.*

---

## 2. Problem Statement

### 2.1 The Problem

AI agents are becoming first-class on-chain actors. A typical scenario:

- 500 agents transact concurrently on Monad (sub-second blocks, parallel EVM)
- Each transaction needs reputation validation (is this agent trustworthy?)
- Naive design: one storage slot per agent's reputation score → every write conflicts
- Result: **~60% abort rate** under 500 concurrency, throughput collapses

### 2.2 Why Existing Solutions Fail

| Approach | Failure mode |
|---|---|
| Single global mapping | Write-write conflicts on every transaction |
| Off-chain reputation DB | Trust assumption, not auditable on-chain |
| Merkle root commitment | Update latency, can't serve real-time validation |
| Multi-chain indexing without reorg safety | Silent data corruption on reorgs |

### 2.3 PrismSettle's Insight

Monad's OCC aborts transactions whose **read/write sets overlap**. If we ensure two transactions touching different agents write to **disjoint storage slots**, the OCC can parallelize them. Sharding by `agentId & 0xFF` achieves this with 256 slots.

---

## 3. Goals & Non-Goals

### 3.1 Goals (in priority order)

| # | Goal | Success metric |
|---|---|---|
| G1 | 256-shard contract deployed on Monad testnet | Contract address live, callable |
| G2 | Abort rate < 10% under 500 concurrent submits | Stress test report comparing V0 vs V1 |
| G3 | Go indexer syncs PrismSettle events with reorg safety | 0 silent data corruption across simulated reorgs |
| G4 | Dashboard visualizes live validation feed + shard heatmap | Pitch-ready, runs against live testnet data |
| G5 | 5-minute pitch tells the agent-economy story | Judge can explain the OCC problem + solution in one sentence |

### 3.2 Non-Goals (explicitly excluded)

- Multi-chain reputation aggregation (out of scope for hackathon)
- Validator slashing automation via on-chain governance (off-chain proof only)
- ZK proofs of validation (proofHash is a hash, not a ZK circuit)
- Production-grade HA deployment (single instance is fine for demo)
- WebSocket push (5s polling is sufficient)

---

## 4. Users & Personas

### 4.1 Primary Personas

**Agent Operator** (e.g., a data-labeling marketplace operator)
- Runs 500+ AI agents that transact on Monad
- Needs to verify agent reputation before each transaction
- Pain: transactions abort due to OCC conflicts → throughput collapse

**Validator** (stake-holding reputation provider)
- Locks 100+ MON as stake
- Submits validation scores for agents
- Earns fees, risks slash for Sybil coordination

**Recruiter / Judge** (secondary)
- Looks at the GitHub repo and dashboard for 5 minutes
- Needs to grasp "this is solid infra work" in under 60 seconds

### 4.2 Anti-Persona

- **Retail DeFi user** — PrismSettle is infra, not a consumer product. No wallet UI for end users.

---

## 5. Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         Monad Testnet                       │
│                                                             │
│   ┌─────────────────────────────────────────────────────┐  │
│   │           PrismSettleRegistry.sol                  │  │
│   │                                                     │  │
│   │   submitValidation() ──→ shardValidations[256]     │  │
│   │                              │                     │  │
│   │   aggregateEpoch() ──────────┼──→ aggregatedScore  │  │
│   │                              │      (low-freq)     │  │
│   │   stake() / slash()           │                     │  │
│   └──────────────────────────────┼─────────────────────┘  │
│                                  │                          │
└──────────────────────────────────┼──────────────────────────┘
                                   │ events
                                   ▼
┌─────────────────────────────────────────────────────────────┐
│                    Go Offchain Indexer                     │
│                                                             │
│  EVMListener ──> Parser Registry ──> EventIngestService     │
│       │              │                      │               │
│   reorg detect    PrismSettleParser     sequential commit  │
│   + rollback      (decodes 5 events)    (nextExpected)     │
│       │                                     │              │
│   LRU block cache                         DB write          │
└──────────────────────────────┬──────────────────────────────┘
                               │ REST API
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                  Next.js Dashboard (frontend)              │
│                                                             │
│   Landing: Prism hologram + narrative                       │
│   Console: Shard heatmap + live feed + agent leaderboard   │
└─────────────────────────────────────────────────────────────┘
```

---

## 6. Functional Requirements

### 6.1 Smart Contract (PrismSettleRegistry.sol)

| FR | Description | Priority |
|---|---|---|
| C1 | `registerAgent(agentId)` — register an agent with owner | P0 |
| C2 | `stake()` — validator locks ≥100 MON | P0 |
| C3 | `submitValidation(agentId, score, proofHash)` — only staked validators, writes to shard | P0 |
| C4 | `aggregateEpoch(agentId)` — rate-limited aggregation, stake-weighted score | P0 |
| C5 | `slash(validator, evidenceHash)` — slash a validator's stake | P1 |
| C6 | `getScore(agentId)` — read latest aggregated score | P0 |
| C7 | `getValidationCount(agentId)` — read count of raw validations | P0 |
| C8 | Event emission for all state changes (5 events) | P0 |

### 6.2 Go Indexer (offchain)

| FR | Description | Priority |
|---|---|---|
| I1 | Listen to PrismSettleRegistry events on Monad testnet | P0 |
| I2 | Parse 5 event types into `ChainEvent` model | P0 |
| I3 | Reorg detection via parent-hash comparison | P0 |
| I4 | Reorg rollback: delete events `block_number > ancestor`, reset `completedTasks` | P0 |
| I5 | Sequential commit via `nextExpected`/`completedTasks` counter | P0 |
| I6 | LRU block-header cache (1000 entries) | P1 |
| I7 | Graceful shutdown: listener stop → DB close | P1 |
| I8 | Config fail-fast on missing required fields | P0 |

### 6.3 REST API

| FR | Method & Path | Description | Priority |
|---|---|---|---|
| A1 | `GET /api/v1/prismsettle/events` | Paginated event list, filter by agentId/chainName | P0 |
| A2 | `GET /api/v1/prismsettle/score` | Latest aggregated score for an agent | P0 |
| A3 | `GET /api/v1/prismsettle/validations/count` | Validation count for an agent | P0 |
| A4 | `GET /api/v1/prismsettle/shards/activity` | 256-element array of per-shard event counts (for heatmap) | P1 |
| A5 | `GET /health` | Service health: last block, event count, reorg count, uptime | P1 |

### 6.4 Dashboard (frontend)

| FR | Description | Priority |
|---|---|---|
| F1 | Landing page with prism hologram hero | P0 |
| F2 | Console overview: hero metrics + prism + heatmap + feed | P0 |
| F3 | Agent detail page: score history + validation timeline | P1 |
| F4 | Shard heatmap component (16×16 grid, pulsing on new events) | P0 |
| F5 | Live validation feed (5s polling, reorg markers) | P0 |
| F6 | Dark mode default, violet/pink brand palette | P0 |
| F7 | Mobile-responsive (basic) | P2 |

### 6.5 Observability

| FR | Description | Priority |
|---|---|---|
| O1 | Structured logging (zap) with chain/block/tx context | P0 |
| O2 | `/health` endpoint exposing sync lag, reorg count, uptime | P1 |
| O3 | Prometheus metrics endpoint (optional, stretch) | P2 |

---

## 7. Non-Functional Requirements

### 7.1 Performance

| Metric | Target | Measurement |
|---|---|---|
| Contract abort rate @ 500 concurrent | < 10% (vs 60% baseline) | Stress test on anvil + custom load generator |
| Indexer sync lag | < 5 blocks behind head | `/health` endpoint |
| API p99 latency | < 200ms for paginated queries | Locust / k6 |
| Dashboard initial load | < 2s on cold start | Lighthouse |
| Dashboard polling cost | < 5 requests / 5s per active tab | Network tab |

### 7.2 Correctness

| Requirement | Verification |
|---|---|
| No silent data corruption on reorg | Unit test: simulate 3-block reorg, assert DB state matches |
| No duplicate events under retry | Sequential commit counter monotonic |
| Address stored as original EIP-55 | Query test: case-insensitive match returns correct rows |
| Score computation matches contract | Test: hand-computed expected score == `aggregateEpoch` output |

### 7.3 Maintainability

- All code in English (comments, logs, errors, commit messages)
- Tests cover: parser decoding, reorg rollback, sequential commit, score computation
- Each contract module mirrors the `prismsettle/` layout (parser/service/api)
- No backwards-compat shims — break and update

---

## 8. Sprint Plan

### Sprint 1 — Contract & Indexer Closure (current)

**Goal**: end-to-end working pipeline from contract event to API response.

- [x] PrismSettleRegistry.sol — 256-shard design, EPOCH gating, stake-weighted aggregation
- [x] Foundry tests — 21 unit tests covering submit/aggregate/stake/slash
- [x] PrismSettleParser — decodes 5 events into `ChainEvent`
- [x] PrismSettleService + handler — 3 API endpoints
- [x] Config for Monad testnet
- [ ] **TODO**: Deploy contract to Monad testnet, replace placeholder address in `prod.yaml`
- [ ] **TODO**: End-to-end verification: trigger event on-chain, verify it appears in API

### Sprint 2 — Stress Test & Comparison

**Goal**: quantitative proof of the abort rate reduction.

- [ ] Write V0 baseline contract (single-slot, no sharding)
- [ ] Load generator: 500 concurrent `submitValidation` calls on anvil
- [ ] Measure V0 abort rate (~60% expected)
- [ ] Measure V1 abort rate (~5% expected)
- [ ] Write benchmark report with charts (V0 vs V1)
- [ ] Add ShardActivity API endpoint (A4) for heatmap data

### Sprint 3 — Observability & Productionization

**Goal**: service is observable and deployable.

- [ ] `/health` endpoint (O2)
- [ ] Structured logging audit (ensure all hot paths log with context)
- [ ] Dockerfile + docker-compose for one-command deploy
- [ ] README with quickstart (anvil + deploy + indexer + dashboard)
- [ ] Optional: Prometheus metrics (O3)

### Sprint 4 — Dashboard

**Goal**: pitch-ready frontend.

- [ ] Next.js + Tailwind v4 + shadcn/ui setup
- [ ] PrismHologram component (CSS 3D + Framer Motion)
- [ ] ShardHeatmap component (16×16 pulsing grid)
- [ ] ValidationFeed component (live ticker, reorg-aware)
- [ ] Landing page (GSAP scroll-triggered sections)
- [ ] Console overview page
- [ ] Agent detail page (score history chart)

### Sprint 5 — Narrative Packaging & Pitch

**Goal**: 5-minute pitch that lands.

- [ ] Demo script: anvil → deploy → simulate 500 agents → show dashboard
- [ ] Pitch deck (5 slides max)
- [ ] Recorded demo video (90s)
- [ ] README polish with screenshots
- [ ] Twitter/X thread announcing the project

---

## 9. Success Metrics

### 9.1 Technical

| Metric | Target |
|---|---|
| Abort rate reduction | 60% → < 10% |
| Test coverage on core modules | > 80% |
| Reorg rollback correctness | 100% (no silent corruption) |
| API uptime during demo | 100% |

### 9.2 Narrative

| Metric | Target |
|---|---|
| Judge can explain the problem in 1 sentence | After 60s of pitch |
| Recruiter sees test coverage + reorg handling in README | < 5 min scan |
| Dashboard visually distinct from generic hackathon dashboards | Prism hologram + shard heatmap |

---

## 10. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Monad testnet RPC unstable | Medium | High | Add retry with backoff in listener; cache aggressively |
| Contract deployment fails (gas/format) | Low | High | Test on anvil first; use `forge script` |
| Abort rate not as low as predicted | Medium | Medium | V0 baseline gives us a comparison even if V1 is 15% not 5% |
| Dashboard takes too long to build | Medium | Medium | Use shadcn/ui primitives; skip mobile; poll not websocket |
| Pitch too technical for judges | Medium | High | Lead with the agent-economy story, not the OCC internals |

---

## 11. Out of Scope (Future Work)

- Multi-chain reputation aggregation
- On-chain slashing governance (off-chain proof only for now)
- ZK proofs of validation
- Validator discovery protocol
- Agent SDK (agents call the contract directly)
- Mobile app
- WebSocket real-time push

---

## 12. Glossary

| Term | Definition |
|---|---|
| **OCC** | Optimistic Concurrency Control — Monad's parallel execution model, aborts txs with conflicting read/write sets |
| **Shard** | One of 256 storage slots, selected by `agentId & 0xFF` |
| **EPOCH** | Minimum interval (1 minute) between two `aggregateEpoch` calls for the same agent |
| **Validation** | A validator's score submission for an agent (0..1e18 fixed-point) |
| **Aggregation** | Stake-weighted average of all validations for an agent, computed on the slow path |
| **Reorg** | Blockchain reorganization — blocks get orphaned, events must be rolled back |
| **Sequential commit** | Monotonic counter ensuring exactly-once event ingestion |

---

## 13. Open Questions

| # | Question | Owner | Status |
|---|---|---|---|
| Q1 | Is 256 the optimal shard count for 500 concurrent agents? | Contract | Resolved (birthday bound) |
| Q2 | Should we use EIP-1153 transient storage for the hot path? | Contract | Deferred (start with regular storage) |
| Q3 | How to demo reorgs convincingly in 5 minutes? | Demo | Open — anvil `evm_reorg` possible |
| Q4 | Should the dashboard support wallet connect for validators? | Frontend | Deferred (read-only is enough for pitch) |

---

## 14. References

- PrismSettleRegistry.sol: [contracts/src/PrismSettleRegistry.sol](file:///home/administrator/Documents/trae_projects/PrismSettle/contracts/src/PrismSettleRegistry.sol)
- Foundry tests: [contracts/test/PrismSettleRegistry.t.sol](file:///home/administrator/Documents/trae_projects/PrismSettle/contracts/test/PrismSettleRegistry.t.sol)
- Go indexer: [offchain/](file:///home/administrator/Documents/trae_projects/PrismSettle/offchain/)
- API endpoints: [offchain/prismsettle/api/prismsettle_handler.go](file:///home/administrator/Documents/trae_projects/PrismSettle/offchain/prismsettle/api/prismsettle_handler.go)
- Project conventions: [.trae/skills/prismsettle-conventions/SKILL.md](file:///home/administrator/Documents/trae_projects/PrismSettle/.trae/skills/prismsettle-conventions/SKILL.md)
- UI patterns: [.trae/skills/prismsettle-ui-dashboard/SKILL.md](file:///home/administrator/Documents/trae_projects/PrismSettle/.trae/skills/prismsettle-ui-dashboard/SKILL.md)

---

**End of Document**
