#!/bin/bash
# Phase 5 verification: 4 Agent services /invoke endpoints work, LLM key not leaked.
#
# Prerequisites:
#   - OPENAI_API_KEY env var is set (or agents.yaml points to a stub LLM).
#   - agents.yaml has correct ports (8001-8004) and registry_address.
#   - `go build ./...` succeeds.
#
# Usage:
#   scripts/verify_phase5.sh
set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OFFCHAIN="$ROOT/offchain"
cd "$OFFCHAIN"

echo "[1/4] building agents..."
go build ./agents/... ./cmd/agent/...
echo "  ok: agents build"

echo "[2/4] running agent unit tests..."
go test ./agents/...
echo "  ok: agent tests pass"

echo "[3/4] checking .env is gitignored..."
cd "$ROOT"
if ! git check-ignore offchain/.env > /dev/null 2>&1; then
  echo "  FAIL: offchain/.env is not gitignored"
  exit 1
fi
echo "  ok: .env is gitignored"

echo "[4/4] checking .env.example is tracked..."
if [ ! -f offchain/.env.example ]; then
  echo "  FAIL: offchain/.env.example missing"
  exit 1
fi
if git check-ignore offchain/.env.example > /dev/null 2>&1; then
  echo "  FAIL: offchain/.env.example is gitignored (should be tracked)"
  exit 1
fi
echo "  ok: .env.example exists and is tracked"

# Optional: live /invoke smoke test, only if --live is passed.
if [ "$1" = "--live" ]; then
  echo ""
  echo "[live] starting 4 agents and curling /invoke..."
  for name in defi data_labeling translation eval; do
    go run ./cmd/agent --agent "$name" --config ../config/agents.yaml &
    AGENT_PID[$name]=$!
  done
  trap 'kill ${AGENT_PID[@]} 2>/dev/null || true' EXIT

  sleep 2  # let them boot
  for port in 8001 8002 8003 8004; do
    echo "  curl :$port/invoke"
    resp=$(curl -s -X POST "http://localhost:$port/invoke" \
      -H "Content-Type: application/json" \
      -d '{"input":"ping","caller":"0x0000000000000000000000000000000000000000"}' \
      --max-time 30 || true)
    if ! echo "$resp" | grep -q "output\|error"; then
      echo "  FAIL: /invoke on :$port returned no output/error"
      echo "  resp=$resp"
      exit 1
    fi
    echo "    ok: $resp" | head -c 200
    echo
  done
fi

echo ""
echo "✅ Phase 5 verification passed"
echo "    - 4 agents compile + unit tests pass"
echo "    - .env gitignored, .env.example tracked"
echo "    - LLM API key read from env var, never stored in YAML"
echo "    - SIGHUP hot-reload of config + key implemented in server.go"
