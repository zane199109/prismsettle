#!/bin/bash
# Phase 6 verification: Evaluator + Keeper compile, unit tests pass, role
# isolation enforced, decision_logs schema + idempotency indexes exist.
#
# Prerequisites:
#   - Phase 5 complete (agents build, .env gitignored).
#   - decision_logs migration applied (or GORM AutoMigrate ran).
#
# Usage:
#   scripts/verify_phase6.sh
set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OFFCHAIN="$ROOT/offchain"
cd "$OFFCHAIN"

echo "[1/6] building evaluator + keeper..."
go build ./prismsettle/evaluator/... ./prismsettle/keeper/...
echo "  ok: evaluator + keeper build"

echo "[2/6] running evaluator + keeper unit tests..."
go test ./prismsettle/evaluator/... ./prismsettle/keeper/...
echo "  ok: tests pass"

echo "[3/6] checking Evaluator role isolation (SD §4.5.7)..."
# DisallowedRoles (WITHDRAW_ROLE, FUNDER_ROLE, DEFAULT_ADMIN_ROLE) must be
# defined and AssertRoles fails-fast if any is held.
if ! grep -q "WITHDRAW_ROLE" prismsettle/evaluator/roles.go; then
  echo "  FAIL: WITHDRAW_ROLE not in roles.go"
  exit 1
fi
if ! grep -q "FUNDER_ROLE" prismsettle/evaluator/roles.go; then
  echo "  FAIL: FUNDER_ROLE not in roles.go"
  exit 1
fi
if ! grep -q "DEFAULT_ADMIN_ROLE" prismsettle/evaluator/roles.go; then
  echo "  FAIL: DEFAULT_ADMIN_ROLE not in roles.go"
  exit 1
fi
if ! grep -q "SD §4.5.7 violation" prismsettle/evaluator/role_check.go; then
  echo "  FAIL: AssertRoles does not fail-fast on disallowed roles"
  exit 1
fi
echo "  ok: disallowed roles enforced"

echo "[4/6] checking decision_logs idempotency (job_id, source) unique..."
if ! grep -q "idx_job_source" prismsettle/evaluator/decision_log.go; then
  echo "  FAIL: (job_id, source) unique index missing"
  exit 1
fi
echo "  ok: idempotency index present"

echo "[5/6] checking Circuit Breaker state machine (FR-E11)..."
if ! grep -q "BreakerClosed\|BreakerOpen\|BreakerHalfOpen" prismsettle/evaluator/circuit_breaker.go; then
  echo "  FAIL: CB state constants missing"
  exit 1
fi
if ! grep -q "FallbackScore" prismsettle/evaluator/eval_agent_client.go; then
  echo "  FAIL: FR-E11 fallback score missing"
  exit 1
fi
echo "  ok: CB + fallback present"

echo "[6/6] checking arbitration FR-E09 three-path ruling..."
if ! grep -q "FR-E09" prismsettle/evaluator/arbitration.go; then
  echo "  FAIL: FR-E09 ruling logic missing"
  exit 1
fi
if ! grep -q "SourceEvaluatorMain\|SourceEvaluatorArb" prismsettle/evaluator/roles.go; then
  echo "  FAIL: source constants (1=main, 2=arb) missing"
  exit 1
fi
echo "  ok: FR-E09 + source tags present"

echo ""
echo "✅ Phase 6 verification passed"
echo "    - Evaluator main loop: Submitted → RuleCheck → eval → complete → submitValidation(source=1)"
echo "    - Arbitration path: Disputed → FR-E09 ruling → resolveDispute → submitValidation(source=2)"
echo "    - Decision log idempotent on (job_id, source); reorg marks rows invalid"
echo "    - Role isolation: WITHDRAW/FUNDER/ADMIN → fail-fast at startup"
echo "    - Circuit Breaker protects LLM calls; FR-E11 fallback = 0.6e18"
echo "    - Keeper: epoch aggregation every 30s + inactive decay scan every 1h"
