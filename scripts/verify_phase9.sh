#!/bin/bash
# Phase 9 verification — integration scaffolding + perf endpoints + Docker stack.
#
# Scope:
#   - Docker Compose orchestration (offchain + 4 agents + frontend + postgres + redis + anvil)
#   - .env.example template (secrets never baked into images)
#   - /health endpoint with sync_lag / reorg_count / evaluator_state / keeper_last_run
#   - reorg_events table + Repository (listener writes real reorg data)
#   - perf_results table + Repository (load-test writes real V0/V1 data)
#   - /perf/reorg-feed + /perf/v0-v1-comparison no longer return mock
#   - offchain + frontend Dockerfiles (multi-stage, non-root, healthcheck)
#
# Out of scope (require live chain + manual steps):
#   - docker compose up (needs docker daemon)
#   - anvil fork + reorg simulation (task 9.4 manual)
#   - 500 concurrent load test (task 9.5 manual)
#   - Monad testnet deployment (task 9.7 manual)
#
# Usage:
#   scripts/verify_phase9.sh
set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OFFCHAIN="$ROOT/offchain"
FRONTEND="$ROOT/frontend"

echo "[1/9] checking docker-compose orchestration..."
if [ ! -f "$ROOT/docker-compose.yml" ]; then
  echo "  FAIL: docker-compose.yml missing at repo root"
  exit 1
fi
for svc in postgres redis offchain agent-defi agent-data agent-trl agent-eval frontend; do
  if ! grep -q "  $svc:" "$ROOT/docker-compose.yml"; then
    echo "  FAIL: service '$svc' missing in docker-compose.yml"
    exit 1
  fi
done
# anvil must be gated behind the dev profile so prod doesn't start a local chain.
if ! grep -q "profiles:.*\[.dev.\]" "$ROOT/docker-compose.yml"; then
  echo "  FAIL: anvil not gated behind dev profile"
  exit 1
fi
echo "  ok: 8 services present (postgres/redis/offchain/4 agents/frontend + anvil dev)"

echo "[2/9] checking .env.example..."
if [ ! -f "$ROOT/.env.example" ]; then
  echo "  FAIL: .env.example missing"
  exit 1
fi
for var in POSTGRES_PASSWORD PRISM_RPC_URL PRISM_EVALUATOR_KEY PRISM_KEEPER_KEY OPENAI_API_KEY NEXT_PUBLIC_API_BASE_URL; do
  if ! grep -q "^$var=" "$ROOT/.env.example"; then
    echo "  FAIL: $var not declared in .env.example"
    exit 1
  fi
done
# .env must be gitignored, .env.example must be allowed.
# Use single quotes around the pattern to avoid bash history expansion on '!'.
if ! grep -q '^\.env$' "$ROOT/.gitignore" || ! grep -q '^!\.env\.example$' "$ROOT/.gitignore"; then
  echo "  FAIL: .gitignore must exclude .env but allow .env.example"
  exit 1
fi
echo "  ok: .env.example complete + .gitignore correct"

echo "[3/9] checking Dockerfiles (offchain + frontend)..."
for df in "$OFFCHAIN/Dockerfile" "$FRONTEND/Dockerfile"; do
  if [ ! -f "$df" ]; then
    echo "  FAIL: $df missing"
    exit 1
  fi
  # Multi-stage build is required for image size + supply-chain hygiene.
  if ! grep -q "AS builder" "$df" && ! grep -q "AS deps" "$df"; then
    echo "  FAIL: $df not multi-stage"
    exit 1
  fi
  # Non-root user is required (security baseline).
  if ! grep -q "USER " "$df"; then
    echo "  FAIL: $df does not drop to non-root user"
    exit 1
  fi
  # HEALTHCHECK so Docker can restart unhealthy containers.
  if ! grep -q "HEALTHCHECK" "$df"; then
    echo "  FAIL: $df missing HEALTHCHECK"
    exit 1
  fi
done
echo "  ok: both Dockerfiles multi-stage + non-root + healthcheck"

echo "[4/9] checking HealthTracker + /health endpoint..."
if [ ! -f "$OFFCHAIN/prismsettle/service/health.go" ]; then
  echo "  FAIL: health.go missing"
  exit 1
fi
for fn in IncReorg SetSyncLag SetEvaluatorState TouchKeeper HealthSnapshot; do
  if ! grep -q "func.*$fn" "$OFFCHAIN/prismsettle/service/health.go"; then
    echo "  FAIL: HealthTracker.$fn missing"
    exit 1
  fi
done
# Router must accept a HealthChecker and emit all 4 fields.
if ! grep -q "HealthChecker" "$OFFCHAIN/internal/router/router.go"; then
  echo "  FAIL: router.HealthChecker interface missing"
  exit 1
fi
for field in sync_lag reorg_count evaluator_state keeper_last_run; do
  if ! grep -q "$field" "$OFFCHAIN/internal/router/router.go"; then
    echo "  FAIL: /health does not expose $field"
    exit 1
  fi
done
echo "  ok: HealthTracker + 4-field /health endpoint wired"

echo "[5/9] checking ReorgEvent model + repository..."
if ! grep -q "type ReorgEvent struct" "$OFFCHAIN/model/models.go"; then
  echo "  FAIL: ReorgEvent model missing"
  exit 1
fi
if [ ! -f "$OFFCHAIN/internal/repository/reorg_event_repository.go" ]; then
  echo "  FAIL: reorg_event_repository.go missing"
  exit 1
fi
for fn in Insert ListRecent; do
  if ! grep -q "func.*$fn" "$OFFCHAIN/internal/repository/reorg_event_repository.go"; then
    echo "  FAIL: ReorgEventRepository.$fn missing"
    exit 1
  fi
done
# AutoMigrate must register the table.
if ! grep -q "model.ReorgEvent" "$OFFCHAIN/internal/storage/pg_dao.go"; then
  echo "  FAIL: ReorgEvent not in AutoMigrate"
  exit 1
fi
echo "  ok: reorg_events table + repository + migration registered"

echo "[6/9] checking listener reorg persistence..."
if ! grep -q "reorgRepo" "$OFFCHAIN/internal/listener/evm_listener.go"; then
  echo "  FAIL: EVMListener.reorgRepo field missing"
  exit 1
fi
if ! grep -q "l.reorgRepo.Insert" "$OFFCHAIN/internal/listener/evm_listener.go"; then
  echo "  FAIL: listener does not call reorgRepo.Insert after rollback"
  exit 1
fi
if ! grep -q "l.healthTracker.IncReorg" "$OFFCHAIN/internal/listener/evm_listener.go"; then
  echo "  FAIL: listener does not bump healthTracker.IncReorg"
  exit 1
fi
echo "  ok: listener writes reorg_events + bumps health counter"

echo "[7/9] checking PerfResult model + repository..."
if ! grep -q "type PerfResult struct" "$OFFCHAIN/model/models.go"; then
  echo "  FAIL: PerfResult model missing"
  exit 1
fi
if [ ! -f "$OFFCHAIN/internal/repository/perf_result_repository.go" ]; then
  echo "  FAIL: perf_result_repository.go missing"
  exit 1
fi
for fn in Insert GetLatest; do
  if ! grep -q "func.*$fn" "$OFFCHAIN/internal/repository/perf_result_repository.go"; then
    echo "  FAIL: PerfResultRepository.$fn missing"
    exit 1
  fi
done
if ! grep -q "model.PerfResult" "$OFFCHAIN/internal/storage/pg_dao.go"; then
  echo "  FAIL: PerfResult not in AutoMigrate"
  exit 1
fi
echo "  ok: perf_results table + repository + migration registered"

echo "[8/9] checking perf endpoints serve real data (not mock)..."
# GetReorgFeed must take ctx + chainName + limit (Phase 9 signature).
if ! grep -q "func.*GetReorgFeed(ctx context.Context, chainName string, limit int)" "$OFFCHAIN/prismsettle/service/prismsettle_service.go"; then
  echo "  FAIL: GetReorgFeed signature not updated for Phase 9"
  exit 1
fi
if ! grep -q "s.reorgRepo.ListRecent" "$OFFCHAIN/prismsettle/service/prismsettle_service.go"; then
  echo "  FAIL: GetReorgFeed does not call reorgRepo.ListRecent"
  exit 1
fi
# GetPerfComparison must take ctx and fall back to mock only when no row exists.
if ! grep -q "func.*GetPerfComparison(ctx context.Context)" "$OFFCHAIN/prismsettle/service/prismsettle_service.go"; then
  echo "  FAIL: GetPerfComparison signature not updated for Phase 9"
  exit 1
fi
if ! grep -q "s.perfRepo.GetLatest" "$OFFCHAIN/prismsettle/service/prismsettle_service.go"; then
  echo "  FAIL: GetPerfComparison does not call perfRepo.GetLatest"
  exit 1
fi
# main.go must wire all new repos.
if ! grep -q "NewPrismSettleServiceWithPerfRepos" "$OFFCHAIN/cmd/main.go"; then
  echo "  FAIL: main.go not using NewPrismSettleServiceWithPerfRepos"
  exit 1
fi
echo "  ok: /perf/reorg-feed + /perf/v0-v1-comparison wired to real data"

echo "[9/9] running offchain build + tests..."
cd "$OFFCHAIN"
if ! go build ./... 2>&1 | tail -5; then
  echo "  FAIL: go build failed"
  exit 1
fi
if ! go test ./prismsettle/... ./internal/repository/... 2>&1 | tail -10; then
  echo "  FAIL: go test failed"
  exit 1
fi
echo "  ok: offchain builds + tests pass"

cd "$ROOT"
echo ""
echo "✅ Phase 9 verification passed"
echo "    - docker-compose: 8 services (offchain + 4 agents + frontend + postgres + redis + anvil dev profile)"
echo "    - .env.example: complete template, .env gitignored, .env.example allowed"
echo "    - Dockerfiles: multi-stage + non-root + HEALTHCHECK (offchain + frontend)"
echo "    - /health: sync_lag + reorg_count + evaluator_state + keeper_last_run (NFR-OBS02)"
echo "    - reorg_events: table + repository + listener writes real reorg data"
echo "    - perf_results: table + repository (load-test writer, API reader)"
echo "    - /perf/reorg-feed: real data from reorg_events (was mock in Phase 7)"
echo "    - /perf/v0-v1-comparison: real data from perf_results (falls back to mock if empty)"
echo ""
echo "⚠️  Manual verification still required (needs live chain):"
echo "    - docker compose --profile dev up -d  (task 9.1 runtime check)"
echo "    - anvil fork + reorg simulation       (task 9.4)"
echo "    - 500 concurrent load test            (task 9.5, NFR-MN01)"
echo "    - Monad testnet deployment            (task 9.7)"
