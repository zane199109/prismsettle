# PrismSettle agent instructions

## Working rules

- **All code comments and interactions must be in English.** User-facing
  communication follows the user's language.
- **Go module path is `github.com/zane/web3-offchain`** — never change.
- **Go 1.25+ is required** due to go-ethereum v1.13.14 and quic-go v0.59.0.
  Do not downgrade to 1.22.
- **Contracts use Solidity 0.8.24 + Foundry**. Job and Hook contracts must
  use OpenZeppelin standard library for role management and token interfaces.
- **256-shard storage is a hard constraint.** Shard formula: `agentId & 0xFF`.
  Never revert to single-slot storage.
- **jobId uses uint256** for keccak256 output compatibility and arithmetic
  operations. Do not switch to bytes32.
- **ERC-8183 state machine is immutable**: the official 4 macro-states
  (Open/Funded/Submitted/Terminal) map onto 7 concrete `JobState` values
  (`Created`, `Funded`, `Assigned`, `Submitted`, `DisputeResolved`,
  `Completed`, `Refunded`). `Assigned` refines the pre-submit stage and
  `DisputeResolved` is a settlement refinement of Terminal entered via
  the `notifyDisputeResolved` callback from the Hook. Arbitration logic
  (dispute, arbitrator selection, ruling) stays in the optional
  ArbitrationHook contract, never in Job. After an `ANNOUNCEMENT_PERIOD`
  (1 hour), anyone can call `executeArbitrationResult` to move to
  Completed or Refunded.
- **submitValidation signature** must include: `(agentId, score, proofHash,
  jobId, source)`. source values: 0=Validator, 1=Evaluator, 2=Arbitration.
- **Testing must be split into two independent files**: `ArbitrationHook.t.sol`
  and `PrismSettleJob.t.sol`. Do not merge.
- **Frontend npm installation must use domestic registry**
  (`https://registry.npmmirror.com`) to prevent network timeouts.
- **offchain Dockerfile must set GOPROXY** to `https://goproxy.cn,direct`.
- **Docker Compose**: anvil is in `dev` profile, not started by default. Use
  `--profile dev` for local testing.
- **Nginx is not required.** Next.js handles HTTP server and API rewrites to
  offchain:9527.
- **Delete, don't deprecate.** When a refactor supersedes code, replace it
  wholesale — no aliases, no re-exports, no shims.
- **Any plan changes must be synchronized and confirmed by the user before
  execution.** Do not act without authorization.
- **Deployment order must follow**: Token → Registry → Hook → Job →
  Hook.setJobContract → Job.setRegistry → grantRoles → seed 4 agents
  (0x1111~0x4444) → register Provider agent (0x5555) → register arbitrator.
  Never deviate from this sequence.

## Review guidelines

- **Review against accepted architecture**, not the diff in isolation. Read
  `CONTEXT.md` and `docs/PRD.zh-CN.v1.0.md` / `docs/SD.zh-CN.v1.0.md` for
  vocabulary and ownership boundaries.
- **Severity classification**:
  - P0: blocking bugs (compile errors, test failures, security vulnerabilities)
  - P1: business correctness (logic errors, missing features)
  - P2: concurrency safety (race conditions, reorg handling)
  - P3: code quality (naming, comments, structure)

### Contracts

- After any contract change, run `forge test` — all 131 tests must pass.
- `MIN_STAKE = 5 ether` (testnet-appropriate; PRD says 100 ether but 5 is the
  implemented value).
- `MAX_VALIDATIONS_PER_EPOCH = 50` for Validator (source=0); Evaluator
  (source≠0) is exempt.
- `aggregateEpoch` processes max 100 records per Epoch to prevent gas limit
  issues.
- Reputation decay: 30 days of inactivity triggers 0.01e18 daily penalty,
  calculated on read via `getScore`.
- Arbitration penalty: `max(0.2e18, currentScore * 30%)`.
- `createJob` includes `minProviderReputation` param — provider must have
  score ≥ this threshold to `grabJob`. Frontend and contract both enforce
  this check.
- Multi-currency escrow: `createJob`'s 6th arg `paymentToken` selects the
  job's token (address(0) = contract-default USDC; any ERC-20 such as WMON
  works). All settlement (`complete`/`claimRefund`/`executeArbitrationResult`)
  and Hook deposits resolve the job's token via `getJobPaymentToken`. The
  x402 receipt path only settles the default token.
- `grabJob(jobId, providerAgentId)` is the Provider self-assignment function.
  Reverts if job is not in Funded state, already assigned, provider
  reputation is below `minProviderReputation`, or the caller is the buyer
  (anti self-dealing — prevents completion-reward farming).
- **Reject / resubmit loop**: buyer may call `reject(jobId, reasonHash)`
  (posts a reject deposit = amount × 5%) to request rework; the provider can
  then `submit` again (Submitted → Submitted, resets `submittedAt` and the
  dispute window). Rejects have no count limit — abuse is bounded by the
  deposit and the provider's arbitration right.
- `executeArbitrationResult` requires `ANNOUNCEMENT_PERIOD` (1 hour) after
  `DisputeResolved` before execution. Payout is the **full escrow** to the
  winner; the arbitrator fee is covered by forfeited deposits, never by escrow.
- Registry has `setAggregatedScore(agentId, newScore)` for Evaluator to
  manually set scores (used for arbitration source=2 results).
- **Reputation dual-factor** in `aggregateEpoch` (source=3 Buyer ratings do
  NOT enter the weighted average):
  - Completion bonus: +0.005e18 per completed job, capped at +0.05e18/epoch
    (activity incentive, anti-sybil).
  - Rating nudge: (score − 0.5e18) × 0.05, capped at ±0.02e18/epoch
    (reference signal, not a verdict).
  - source=0/1/2 keep the EMA (fixed alpha = 0.3e18) weighted path; total
    score is capped at 1e18.
- ArbitrationHook implements an **arbitration pool with deposits**:
  - `registerArbitrator(agentId, feeBps, feeRecipient)` — requires agent
    reputation ≥ 0.7e18 (checked via Registry).
  - `unregisterArbitrator()` — removes self from pool.
  - Highest-reputation registered arbitrator is selected at dispute time.
  - **Dispute**: either party (buyer OR provider) may call `dispute` within
    `DISPUTE_WINDOW` (24h) of the latest submit. Both parties post a deposit
    (amount × 5%, `DEPOSIT_BPS=500`).
  - **Deposit settlement at resolveDispute**: the LOSING side's deposit(s)
    pay the arbitrator (feeRecipient); the winner's deposit(s) are returned.
    Reject deposits follow the same rule (forfeited if the buyer loses,
    returned if the job settles without arbitration).
  - `getArbitratorFeeConfig(jobId)` returns (feeBps, recipient).

### Offchain (Go)

- After any Go change, run `go test ./...` in `offchain/`.
- Listener must use setter methods (`SetReorgRepo`/`SetHealthTracker`) instead
  of constructor parameters for backward compatibility.
- `decision_logs` table includes `invalid` boolean field for reorg-affected
  records; Evaluator queries must filter `invalid = false`.
- Circuit breaker in HALF_OPEN state uses `halfOpenInflight` mutex to allow
  only one probe request.
- `lastActivity` in `AgentMetadata` is only updated during `submitValidation`,
  not during `aggregateEpoch`.
- Perf endpoints must gracefully degrade to mock data when `perf_results`
  table is empty, with a `source` field indicating data origin.

### Frontend

- After any frontend change, run `npx tsc --noEmit` and
  `npx eslint app components --quiet` — both must report 0 errors.
- Tailwind v4 with CSS variables. Brand color: `--prism-accent` (violet).
  Trust color: `--trust` (#f59e0b, gold).
- Fonts: Orbitron (headings) + Exo 2 (body) via `next/font/google`.
- Wallet: wagmi v2 + viem v2 + RainbowKit. Chain: Monad Testnet (chainId 10143).
- Contract addresses centralized in `frontend/lib/contracts.ts`.
- `next.config.js` `output: "standalone"` for Docker builds.
- API rewrite default: `http://localhost:9527` (offchain service port).
