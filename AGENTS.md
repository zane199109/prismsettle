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
- **ERC-8183 core 4-state machine** (Open/Funded/Submitted/Terminal) is
  immutable. Arbitration is implemented as optional Hook contracts, never
  pollute the core state machine.
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

- After any contract change, run `forge test` — all 106 tests must pass.
- `MIN_STAKE = 5 ether` (testnet-appropriate; PRD says 100 ether but 5 is the
  implemented value).
- `MAX_VALIDATIONS_PER_EPOCH = 50` for Validator (source=0); Evaluator
  (source≠0) is exempt.
- `aggregateEpoch` processes max 100 records per Epoch to prevent gas limit
  issues.
- Reputation decay: 30 days of inactivity triggers 0.01e18 daily penalty,
  calculated on read via `getScore`.
- Arbitration penalty: `max(0.2e18, currentScore * 30%)`.

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

## Repo facts

- **Monorepo structure**:
  - `contracts/` — Solidity 0.8.24 + Foundry (106 tests)
  - `offchain/` — Go 1.25, module `github.com/zane/web3-offchain`
  - `frontend/` — Next.js 15 App Router + TypeScript (strict) + Tailwind v4
  - `docs/` — PRD, SD, status review
  - `scripts/` — deployment and verification scripts

- **Contract architecture** (3 core contracts):
  - `PrismSettleRegistry.sol` — Agent registration, staking, validation,
    reputation aggregation (256-shard)
  - `PrismSettleJob.sol` — ERC-8183 Job lifecycle (Created→Funded→Assigned→
    Submitted→Completed/Refunded), x402 funding
  - `ArbitrationHook.sol` — Dispute resolution (Disputed→DisputeResolved),
    independent from Job state machine

- **Offchain architecture** (single binary, port 9527):
  - `internal/listener/` — EVM event listener with reorg detection + rollback
  - `prismsettle/evaluator/` — Rule check + LLM semantic scoring + circuit
    breaker
  - `prismsettle/keeper/` — Periodic `aggregateEpoch` + decay trigger
  - `prismsettle/service/` — 16+ REST API endpoints
  - `agents/` — 4 standalone HTTP agent services (defi/data/trl/eval)
  - `cmd/agent/` — Agent binary entry point

- **Agent services** (4 HTTP servers, shared offchain image):
  - `agent-defi` (port 9101) — DeFi analysis agent
  - `agent-data` (port 9102) — Data labeling agent
  - `agent-trl` (port 9103) — Translation agent
  - `agent-eval` (port 9104) — Evaluation agent (scores deliverables)

- **Docker Compose services**: postgres, redis, anvil (dev profile), offchain,
  agent-defi, agent-data, agent-trl, agent-eval, frontend.

- **Verification commands**:
  - Contracts: `cd contracts && forge test`
  - Offchain: `cd offchain && go test ./...`
  - Frontend: `cd frontend && npx tsc --noEmit && npx eslint app components --quiet`
  - Full stack: `docker compose up -d` (needs `.env` with contract addresses)

- **Deployment sequence**: Hook (with job=0) → Job (with hook address) →
  Hook.setJobContract(Job). Registry permission refactoring must be
  prioritized before implementing Hook and Job contracts.
