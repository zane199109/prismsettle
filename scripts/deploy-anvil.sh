#!/bin/bash
# PrismSettle — Anvil 本地部署 + 自动回填 .env
#
# 用法：
#   bash scripts/deploy-anvil.sh              # 启动 anvil + 部署 + 回填 .env
#   bash scripts/deploy-anvil.sh --skip-anvil  # 跳过启动 anvil（anvil 已在运行）
#
# 部署后自动更新 .env 的 NEXT_PUBLIC_*_ADDRESS，前端 npm run dev 直接可用。

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTRACTS="$ROOT/contracts"
ENV_FILE="$ROOT/.env"
ANVIL_RPC="http://127.0.0.1:8545"

SKIP_ANVIL=false
[ $# -ge 1 ] && [ "$1" = "--skip-anvil" ] && SKIP_ANVIL=true

echo "============================================================"
echo "  PrismSettle — Anvil 本地部署"
echo "============================================================"
echo ""

# ---------- 启动 Anvil ----------
if [ "$SKIP_ANVIL" = false ]; then
  echo "[1/4] 启动 Anvil..."
  pkill anvil 2>/dev/null || true
  sleep 1
  anvil --silent &
  ANVIL_PID=$!
  sleep 2
  # 验证
  cast block-number --rpc-url "$ANVIL_RPC" >/dev/null 2>&1 || {
    echo "  ❌ Anvil 启动失败"
    exit 1
  }
  echo "  Anvil PID: $ANVIL_PID  ✅"
else
  echo "[1/4] 跳过 Anvil 启动（--skip-anvil）"
fi
echo ""

# ---------- 部署合约 ----------
echo "[2/4] 部署合约..."

cd "$CONTRACTS"
FORGE_OUT=$(forge script script/Deploy.s.sol:Deploy \
  --rpc-url "$ANVIL_RPC" \
  --broadcast \
  --legacy \
  --private-key "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80" \
  2>&1) || true

# 检查部署是否真正成功（ONCHAIN EXECUTION COMPLETE）
echo "$FORGE_OUT" | grep -q "ONCHAIN EXECUTION COMPLETE" || {
  echo "  ❌ 链上部署失败"
  echo "$FORGE_OUT" | tail -20
  [ "$SKIP_ANVIL" = false ] && kill $ANVIL_PID 2>/dev/null
  exit 1
}

# 提取合约地址
TOKEN_ADDR=$(echo "$FORGE_OUT" | grep "MockERC20:" | grep -oE '0x[a-fA-F0-9]{40}' | tail -1)
REGISTRY_ADDR=$(echo "$FORGE_OUT" | grep "Registry:" | grep -oE '0x[a-fA-F0-9]{40}' | tail -1)
HOOK_ADDR=$(echo "$FORGE_OUT" | grep "ArbitrationHook:" | grep -oE '0x[a-fA-F0-9]{40}' | tail -1)
JOB_ADDR=$(echo "$FORGE_OUT" | grep "PrismSettleJob:" | grep -oE '0x[a-fA-F0-9]{40}' | tail -1)

echo "  MockERC20:        $TOKEN_ADDR"
echo "  Registry:         $REGISTRY_ADDR"
echo "  ArbitrationHook:  $HOOK_ADDR"
echo "  PrismSettleJob:   $JOB_ADDR"
echo ""

# ---------- 回填 .env ----------
echo "[3/4] 回填 .env..."

python3 - "$ENV_FILE" "$REGISTRY_ADDR" "$JOB_ADDR" "$HOOK_ADDR" "$TOKEN_ADDR" "$ANVIL_RPC" <<'PYEOF'
import re, sys

env_path, reg, job, hook, token, rpc_url = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5], sys.argv[6]

with open(env_path, 'r') as f:
    content = f.read()

replacements = {
    'NEXT_PUBLIC_REGISTRY_ADDRESS=':       f'NEXT_PUBLIC_REGISTRY_ADDRESS={reg}',
    'NEXT_PUBLIC_JOB_CONTRACT_ADDRESS=':   f'NEXT_PUBLIC_JOB_CONTRACT_ADDRESS={job}',
    'NEXT_PUBLIC_HOOK_CONTRACT_ADDRESS=':  f'NEXT_PUBLIC_HOOK_CONTRACT_ADDRESS={hook}',
    'NEXT_PUBLIC_PAYMENT_TOKEN_ADDRESS=':  f'NEXT_PUBLIC_PAYMENT_TOKEN_ADDRESS={token}',
    'NEXT_PUBLIC_RPC_URL=':                f'NEXT_PUBLIC_RPC_URL={rpc_url}',
    'PRISM_RPC_URL=':                      f'PRISM_RPC_URL={rpc_url}',
}

for old, new in replacements.items():
    content = re.sub(rf'^{re.escape(old)}.*', new, content, flags=re.MULTILINE)

with open(env_path, 'w') as f:
    f.write(content)

print("  ✅ .env 已更新（合约地址 + RPC URL）")
PYEOF
echo ""

# ---------- 打印部署信息 ----------
echo "[4/4] 部署完成！当前 .env 状态："
echo ""
grep -E "^(NEXT_PUBLIC_|PRISM_RPC_URL)" "$ENV_FILE" | grep -v "^#" | head -10
echo ""
echo "  前端启动:  cd frontend && npm run dev"
echo "  前端地址:  http://localhost:3000"
echo "  Anvil RPC: $ANVIL_RPC"
echo ""

# 提示用户填入 evaluator/keeper key
if grep -q "^PRISM_EVALUATOR_KEY=$" "$ENV_FILE" 2>/dev/null; then
  echo "  ⚠️  PRISM_EVALUATOR_KEY 和 PRISM_KEEPER_KEY 仍为空，"
  echo "     需要填入才能启动 offchain evaluator 和 keeper。"
  echo ""
fi

echo "============================================================"
