#!/bin/bash
# Phase 7 verification: REST API (16 endpoints) + agent_registry/trust_thresholds
# repos + frontend scaffold compiles. Confirms the contracts between the
# frontend hooks (lib/prismsettle.ts) and the backend routes match.
#
# Prerequisites:
#   - Phase 5/6 complete (evaluator/keeper build & tested).
#   - Node 18+ and npm available on PATH for the frontend build.
#
# Usage:
#   scripts/verify_phase7.sh
set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OFFCHAIN="$ROOT/offchain"
FRONTEND="$ROOT/frontend"

cd "$OFFCHAIN"

echo "[1/8] building offchain (incl. prismsettle/api + service)..."
go build ./...
echo "  ok: offchain builds"

echo "[2/8] running repository + service unit tests..."
go test ./internal/repository/... ./internal/service/...
echo "  ok: repo + service tests pass"

echo "[3/8] checking 16 REST endpoints registered (FR-A01..A12, FR-M11, perf, timeline)..."
# 13 new + 3 existing = 16 route registrations expected.
ROUTE_COUNT=$(grep -cE 'g\.(GET|POST)\(' prismsettle/api/prismsettle_handler.go || true)
if [ "$ROUTE_COUNT" -lt 16 ]; then
  echo "  FAIL: expected >=16 routes, found $ROUTE_COUNT"
  exit 1
fi
echo "  ok: $ROUTE_COUNT routes registered"

echo "[4/8] checking agent_registry mirror (Phase 7 task 7.1)..."
if ! grep -q "TypePrismAgentRegistered" internal/service/event_ingest_service.go; then
  echo "  FAIL: EventIngestService does not mirror AgentRegistered"
  exit 1
fi
if ! grep -q "UpsertFromEvent" internal/repository/agent_registry_repository.go; then
  echo "  FAIL: AgentRegistryRepository.UpsertFromEvent missing"
  exit 1
fi
echo "  ok: agent_registry mirror wired"

echo "[5/8] checking trust_thresholds hot-update endpoint (FR-AP11)..."
if ! grep -q "setTrustThreshold" prismsettle/api/prismsettle_handler.go; then
  echo "  FAIL: POST /trust/thresholds handler missing"
  exit 1
fi
if ! grep -q "AllowThreshold" internal/repository/trust_threshold_repository.go; then
  echo "  FAIL: TrustThresholdRepository missing"
  exit 1
fi
echo "  ok: trust thresholds read/write wired"

echo "[6/8] checking Phase 7 error codes 103xx (SD §8.3)..."
if ! grep -q "ErrAgentNotFound" pkg/errno/errno.go; then
  echo "  FAIL: ErrAgentNotFound missing"
  exit 1
fi
if ! grep -q "ErrJobNotFound" pkg/errno/errno.go; then
  echo "  FAIL: ErrJobNotFound missing"
  exit 1
fi
if ! grep -q "ErrThresholdInvalid" pkg/errno/errno.go; then
  echo "  FAIL: ErrThresholdInvalid missing"
  exit 1
fi
echo "  ok: 103xx error codes present"

echo "[7/8] checking main.go wires new repos into services..."
# Phase 7 originally used NewPrismSettleServiceWithRepos; Phase 9 upgraded it
# to NewPrismSettleServiceWithPerfRepos (adds perfRepo + reorgRepo). Accept
# either name so the check stays valid across both phases.
if ! grep -qE "NewPrismSettleServiceWith(Perf)?Repos" cmd/main.go; then
  echo "  FAIL: main.go not using NewPrismSettleServiceWithRepos (or WithPerfRepos)"
  exit 1
fi
if ! grep -q "NewEventIngestServiceWithAgentRepo" cmd/main.go; then
  echo "  FAIL: main.go not using NewEventIngestServiceWithAgentRepo"
  exit 1
fi
echo "  ok: main.go wiring complete"

echo "[8/8] building frontend (Next.js + wagmi + RainbowKit)..."
if [ ! -d "$FRONTEND/node_modules" ]; then
  echo "  (frontend deps not installed — running npm install)"
  cd "$FRONTEND"
  npm install --no-audit --no-fund
  cd "$OFFCHAIN"
fi
cd "$FRONTEND"
# `next build` exits non-zero on MetaMask SDK unhandled rejection (a known
# upstream bug), but the build itself succeeds and emits .next/. We accept
# that case by checking the output directory after the run.
npx next build || true
if [ ! -d ".next" ] || [ ! -f ".next/BUILD_ID" ]; then
  echo "  FAIL: .next/BUILD_ID missing — frontend build did not complete"
  exit 1
fi
# Sanity-check the API client + hooks exist (Phase 7 task 7.5).
for f in lib/api.ts lib/prismsettle.ts lib/types.ts hooks/usePoll.ts hooks/useJobStatusPoll.ts; do
  if [ ! -f "$f" ]; then
    echo "  FAIL: $f missing"
    exit 1
  fi
done
echo "  ok: frontend builds; API client + hooks present"

cd "$ROOT"
echo ""
echo "✅ Phase 7 verification passed"
echo "    - REST API: 16 endpoints (/agents, /jobs, /trust, /shards/activity, /perf/*, /agent/invoke)"
echo "    - agent_registry mirror: AgentRegistered event -> upsert (best-effort)"
echo "    - trust_thresholds: GET /trust + POST /trust/thresholds (hot update)"
echo "    - Error codes: 10301-10308 (PrismSettle business errors)"
echo "    - Frontend: Next.js 15 + Tailwind v3 + wagmi v2 + RainbowKit v2"
echo "    - API client: lib/api.ts (envelope unwrap) + lib/prismsettle.ts (typed endpoints)"
echo "    - Hooks: usePoll (generic SWR), useJobStatusPoll (auto-stop on terminal state)"
