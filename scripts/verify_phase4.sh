#!/bin/bash
# Phase 4 verification: Indexer core compiles + unit tests pass (reorg /
# sequential commit / parser).
#
# Covers DEV-PLAN Phase 4:
#   - EVMListener: RPC poll + reorg rollback (FR-I01~I03)
#   - Commit ordering: sequential block commit (FR-I04)
#   - Parser: PrismSettle event decoding (FR-I05, FR-JI01~JI04)
#   - DB schema: chain_events / block_state tables
#   - Graceful shutdown: SIGTERM 30s (FR-I07)
#
# Prerequisites:
#   - Phase 0 complete (Go toolchain + foundry).
#   - Postgres available for repository tests (or use testcontainers).
#
# Usage:
#   scripts/verify_phase4.sh
set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OFFCHAIN="$ROOT/offchain"
cd "$OFFCHAIN"

echo "[1/5] building indexer..."
go build ./cmd/...
go build ./internal/...
echo "  ok: indexer builds"

echo "[2/5] go vet..."
go vet ./...
echo "  ok: vet clean"

echo "[3/5] listener tests (reorg rollback + commit ordering)..."
go test ./internal/listener/... -v
echo "  ok: listener tests pass"

echo "[4/5] parser tests (PrismSettle event decoding)..."
go test ./prismsettle/parser/... -v
echo "  ok: parser tests pass"

echo "[5/5] repository + service tests..."
go test ./internal/repository/... ./internal/service/... -v
echo "  ok: repository + service tests pass"

echo ""
echo "✅ Phase 4 verification passed"
echo "    - EVMListener compiles + reorg rollback tested (FR-I03)"
echo "    - Sequential block commit enforced (FR-I04)"
echo "    - PrismSettle event Parser decodes Submitted/Completed/Disputed"
echo "    - DB schema for chain_events / block_state ready"
echo "    - go vet clean"
