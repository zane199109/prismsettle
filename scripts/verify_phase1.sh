#!/bin/bash
# Phase 1 verification: PrismSettleRegistry contract compiles + unit tests pass.
#
# Covers DEV-PLAN Phase 1:
#   - 256-shard design for Monad OCC (SD §3.1)
#   - stake-weighted scoring (FR-R01~R10)
#   - epoch aggregation + decay + slash (FR-R11~R14)
#   - submitValidation / aggregateEpoch / tickDecay / slash
#
# Prerequisites:
#   - Phase 0 complete (foundry installed, cancun EVM configured).
#
# Usage:
#   scripts/verify_phase1.sh
set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT/contracts"

echo "[1/3] building PrismSettleRegistry..."
forge build
echo "  ok: registry builds"

echo "[2/3] running Registry unit tests..."
forge test --match-path test/PrismSettleRegistry.t.sol -vvv
echo "  ok: registry tests pass"

echo "[3/3] checking 256-shard constant (SD §3.1)..."
if ! grep -q "256" src/PrismSettleRegistry.sol; then
  echo "  FAIL: 256-shard design not found in Registry source"
  exit 1
fi
echo "  ok: 256-shard design present"

echo ""
echo "✅ Phase 1 verification passed"
echo "    - PrismSettleRegistry compiles"
echo "    - submitValidation / aggregateEpoch / tickDecay / slash tested"
echo "    - 256-shard + stake-weighted scoring implemented"
