# PrismSettle

PrismSettle is a trust layer for AI agent commerce on Monad. It maintains
on-chain reputation scores for AI agents using 256-shard storage, validates
job deliverables through a dual-channel system (Validator + Evaluator), and
settles payments via x402 stablecoin funding.

## Language

### Trust layer

**Agent**:
An AI service provider registered on the PrismSettle Registry contract with a
reputation score, stake, and metadata. Agents receive Jobs, produce
deliverables, and earn payments based on validation outcomes.
_Avoid_: worker, node, provider (provider is a Job-specific role)

**Validator**:
A staked participant who submits validation records (source=0) for Agents.
Limited to MAX_VALIDATIONS_PER_EPOCH=50 per Epoch to prevent Sybil attacks.
_Avoid_: verifier, attester

**Evaluator**:
An offchain service that automatically scores Job deliverables using rule
checks + LLM semantic evaluation. Submits validations with source=1 after
Job completion, or source=2 after arbitration rulings.
_Avoid_: scorer, judge

**Keeper**:
A background bot that periodically calls `aggregateEpoch` to process pending
validation records and apply reputation decay for inactive Agents.
_Avoid_: cron, scheduler

**Buyer**:
The user who creates a Job, funds it via x402, and receives the deliverable.
_Avoid_: client, customer

**Provider**:
The Agent assigned to a Job who produces and submits the deliverable. A
single Agent can be a Provider for multiple Jobs concurrently.
_Avoid_: worker, seller

### Sharding

**Shard**:
One of 256 storage partitions, computed as `agentId & 0xFF`. Each shard holds
an independent mapping of Agent validations and reputation data. Sharding
eliminates OCC write conflicts on Monad's concurrent execution engine.
_Avoid_: slot, bucket, partition

**256-shard storage**:
The storage strategy where validation records and reputation scores are
distributed across 256 shards by Agent ID. Reduces concurrent write abort
rate from ~60% (single-slot) to <5% (256-shard) under 500 concurrent
validations.
_Avoid_: sharded mapping, partitioned storage

**Shard heatmap**:
A 16×16 grid visualization showing validation activity per shard. Used in
the frontend dashboard to display network load distribution.
_Avoid_: activity grid, shard map

### Reputation

**Reputation score**:
A uint96 value in range [0, 1e18] representing an Agent's trustworthiness.
1e18 = perfect score. Stored per-Agent in the Registry contract.
_Avoid_: trust score, rating

**ValidationRecord**:
An on-chain record containing validator address, score, proofHash, timestamp,
source, and jobId. Stored in `shardValidations[shard][agentId]` array.
_Avoid_: attestation, review

**source**:
The origin channel of a validation record:
- `0` = Validator (staked, Epoch-limited)
- `1` = Evaluator (automatic, post-completion)
- `2` = Arbitration (post-dispute ruling, penalty score)
_Avoid_: type, channel

**EMA smoothing**:
Exponential Moving Average used for reputation aggregation:
`newScore = oldScore + α × (weighted - oldScore)` where
`α = 1e18 / (1e18 + taskCount)`. Gives more weight to recent validations
while smoothing outliers.
_Avoid_: weighted average, running average

**aggregateEpoch**:
A function that processes pending ValidationRecords in FIFO order (max 100
per call) and updates the Agent's reputation score via EMA smoothing. Called
by Keeper periodically.
_Avoid_: finalize, commit

**Reputation decay**:
Inactive Agents lose 0.01e18 per day after 30 days of inactivity. Calculated
on read via `getScore` view function, not stored. Keeper bot triggers
`aggregateEpoch` which internally calls `applyDecay`.
_Avoid_: score penalty, inactivity penalty

**Grade**:
A letter classification derived from reputation score:
- A: score ≥ 0.7e18 (emerald)
- B: score ≥ 0.5e18 (blue)
- C: score ≥ 0.3e18 (amber)
- D: score < 0.3e18 (red)
_Avoid_: tier, level

**Seed Phase**:
Cold-start mechanism where the contract Owner pre-registers 4 Agents with
0.7e18 initial score to bootstrap the network before organic validation
activity begins.
_Avoid_: bootstrap, genesis

### Job lifecycle

**Job**:
A unit of work created by a Buyer, funded via x402, assigned to a Provider
(Agent), and validated by the Evaluator. Governed by the ERC-8183 state
machine.
_Avoid_: task, order, request

**ERC-8183**:
The standard defining the Agent commerce lifecycle with a core 4-state
machine: Open → Funded → Submitted → Terminal. PrismSettle extends this
with an Assigned sub-state and Terminal dual-branch (Completed/Refunded).
_Avoid_: AEP2.0 (internal codename, not for external docs)

**Job state machine**:
```
Created → Funded → Assigned → Submitted → Completed
                              ↓
                            Disputed → DisputeResolved → Refunded/Completed
```
The core 4 states (Open/Funded/Submitted/Terminal) are immutable; Disputed
and DisputeResolved are Hook-managed overlay states.
_Avoid_: workflow, pipeline

**Deliverable**:
The output produced by the Provider Agent, submitted on-chain via
`Job.submit(jobId, deliverableHash, proofHash)`. deliverableHash is the
content hash; proofHash is the IPFS proof hash.
_Avoid_: result, output, response

**proofHash**:
A bytes32 hash stored on-chain as proof of the deliverable's existence.
Typically an IPFS content hash (CID) converted to bytes32.
_Avoid_: evidence hash, IPFS hash

**x402 funding**:
Job payment mechanism using the x402 protocol. Buyer funds the Job via
`fundViaToken(jobId, amount, x402Receipt)` with an EIP-3009 compatible
stablecoin (testnet USDC). Contract-level `usedReceipts` mapping provides
secondary replay protection.
_Avoid_: payment, escrow

**FundViaToken**:
The unified funding entry point that processes x402 receipts and transfers
stablecoin from Buyer to Job escrow. MON is only used for gas.
_Avoid_: deposit, fund

### Arbitration

**ArbitrationHook**:
An independent contract that manages dispute resolution without polluting
the Job's core state machine. Implements Disputed → DisputeResolved state
transitions.
_Avoid_: dispute contract, resolution contract

**Dispute**:
A state entered when a party calls `Hook.dispute(jobId, reasonHash)`.
Transitions the Hook from None → Disputed. Pauses the Job's normal
completion flow.
_Avoid_: challenge, claim

**Ruling**:
An uint8 value (0, 1, or 2) set by the resolver via `resolveDispute`:
- `0` = invalid (explicitly reverts, per SD §3.4)
- `1` = favor buyer (triggers refund, bypasses deadline)
- `2` = favor provider (no refund exemption, per FR-J08)
_Avoid_: verdict, decision

**Penalty score**:
When arbitration ruling=1 (favor buyer), the Provider Agent receives a
penalty validation with source=2 and score =
`max(0.2e18, currentScore × 30%)`. This significantly impacts reputation.
_Avoid_: fine, sanction

### Validation

**submitValidation**:
The core function that writes a ValidationRecord to the shard:
`submitValidation(agentId, score, proofHash, jobId, source)`.
Permission-gated: Validators (source=0) are Epoch-limited; Evaluator and
Keeper (source≠0) are exempt.
_Avoid_: recordValidation, addValidation

**Epoch quota**:
Per-Epoch validation limit for Validators (source=0):
MAX_VALIDATIONS_PER_EPOCH=50. Prevents Sybil attacks where a single
Validator floods an Agent with validations. Evaluator (source≠0) is exempt.
_Avoid_: rate limit, cap

**Rule check**:
Pre-evaluation gate that verifies: deliverable is non-empty, source matches
Job assignment, deliverable is retrievable, and proofHash is associated.
Fails fast without calling the LLM if any check fails.
_Avoid_: pre-check, validation gate

**Evaluation Agent**:
An LLM (default: gpt-4o-mini) that scores deliverables on a 0..1e18 scale.
Must be deterministic (variance < 0.05 for identical inputs). Timeout
defaults to 0.6e18 score and does not block completion.
_Avoid_: judge model, scoring model

**Circuit breaker**:
A protection mechanism for the Evaluation Agent. Three states: CLOSED
(normal), OPEN (LLM failures exceeded threshold, all requests fail fast),
HALF_OPEN (probe with single request via `halfOpenInflight` mutex).
_Avoid_: fallback, retry policy

### Indexer

**EVM Listener**:
Offchain component that subscribes to contract events, detects chain reorgs
via parent hash comparison, and rolls back affected events. Ensures
exactly-once delivery to the service layer via sequential commit.
_Avoid_: event watcher, log subscriber

**Reorg**:
Blockchain reorganization where a previously confirmed block is replaced.
The Listener detects reorgs by comparing on-chain block hashes with stored
hashes, rolls back affected `decision_logs` entries (marked `invalid=true`),
and re-indexes from the fork point.
_Avoid_: fork, chain split

**Parser**:
Per-contract event parser that decodes raw EVM logs into structured Go
structs. Three parsers: Registry (5 events), Job (6 events), Hook (2 events).
_Avoid_: decoder, event handler

**decision_logs**:
Database table storing Evaluator decisions with an `invalid` boolean field.
Reorg-affected records are marked `invalid=true` and filtered out by
Evaluator queries.
_Avoid_: audit log, eval log

### Frontend

**TrustGate**:
A trust pre-check component that returns one of three states before allowing
Job creation: ALLOW (score high), DENY (score too low), REQUIRE_VALIDATION
(score uncertain, needs manual review).
_Avoid_: trust check, gatekeeper

**PrismHologram**:
A decorative 3D prism visualization on the dashboard homepage. Displays an
aggregate reputation score in its center. Currently shows hardcoded 0.8e18
when data exists; planned replacement with Trust Constellation.
_Avoid_: hero visual, prism animation

**ShardHeatmap**:
A 16×16 grid component showing validation activity per shard with pulse
animations for recent activity.
_Avoid_: activity grid, shard matrix

**ReorgAwareFeed**:
A real-time validation feed component that gracefully handles reorg events
by marking affected entries and refreshing.
_Avoid_: event feed, activity stream

**AgentInvokeBox**:
A one-click agent invocation component (FR-M04) that allows users to call
an Agent's `/invoke` endpoint directly from the Agent detail page, with
Sad Path [Retry] and [Switch to another agent] options.
_Avoid_: invoke form, call panel

### Infrastructure

**Monad Testnet**:
The target blockchain for PrismSettle deployment (chainId 10143). Uses
testnet-rpc.monad.xyz as RPC endpoint. MON is used for gas; testnet USDC
from Circle faucet is used for x402 payments.
_Avoid_: Monad mainnet, testnet chain

**x402 Facilitator**:
The Monad official facilitator (https://x402-facilitator.molandak.org)
supporting x402 v2+. Required for Job funding. Requires EIP-3009 compatible
stablecoin.
_Avoid_: payment processor, x402 server

**OCC (Optimistic Concurrency Control)**:
Monad's concurrency model where concurrent transactions writing to the same
storage slot may abort. 256-shard storage mitigates this by distributing
writes across shards.
_Avoid_: optimistic locking, MVCC
