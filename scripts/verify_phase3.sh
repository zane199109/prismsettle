#!/bin/bash
# Phase 3 verification: end-to-end integration test across all 3 contracts.
#
# Covers DEV-PLAN Phase 3:
#   - Integration test: Registry ↔ Job ↔ ArbitrationHook wired together
#     (FR-J01~J08 + FR-R01~R14 + FR-E09 cross-contract flow)
#   - V0/V1 baseline comparison scaffold (BaselineRegistry.t.sol)
#   - Stress test is manual (anvil + bench script) — only flagged here.
#
# Prerequisites:
#   - Phase 1 + 2 complete (all 3 contracts + unit tests pass).
#
# Usage:
#   scripts/verify_phase3.sh
set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT/contracts"

echo "[1/3] building all contracts (incl. integration test fixtures)..."
forge build
echo "  ok: all contracts build"

echo "[2/3] running Integration test (end-to-end flow)..."
forge test --match-path test/Integration.t.sol -vvv
echo "  ok: integration test passes"

echo "[3/3] running V0/V1 baseline comparison test..."
forge test --match-path test/BaselineRegistry.t.sol -vvv
echo "  ok: baseline (V0) registry test passes"

echo ""
echo "✅ Phase 3 verification passed"
echo "    - End-to-end flow: create → fund → assign → submit → complete → submitValidation → aggregateEpoch"
echo "    - Dispute → resolveDispute → claimRefund path covered"
echo "    - V0 BaselineRegistry compiles for V0/V1 benchmarking"
echo ""
echo "⚠️  Stress test (anvil) is manual: bash scripts/bench/v0_v1_bench.sh"
echo "⚠️  Pass bar: V1 abort rate < 5% (NFR-MN01)"
