#!/bin/bash
# PrismSettle — Monad Testnet 部署脚本
#
# 作用：将 PrismSettle 合约部署到 Monad Testnet，自动回填环境变量。
#
# 前置条件：
#   1. 已安装 foundry (forge, cast)
#   2. 部署钱包已通过 faucet 领取 ≥ 1 MON
#      Faucet: https://faucet.monad.xyz/
#
# 用法：
#   bash scripts/deploy_monad_testnet.sh
#
# 部署成功后自动：
#   - 输出 4 个合约地址
#   - 更新 .env 文件的 NEXT_PUBLIC_*_ADDRESS
#   - 生成 docs/testnet-deploy-log.md

set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTRACTS="$ROOT/contracts"
ENV_FILE="$ROOT/.env"
DEPLOY_LOG="$ROOT/docs/testnet-deploy-log.md"

MONAD_RPC="https://testnet-rpc.monad.xyz"
MONAD_CHAIN_ID=10143
MONAD_EXPLORER="https://testnet.monadexplorer.com"

echo "============================================================"
echo "  PrismSettle — Monad Testnet 部署"
echo "============================================================"
echo ""
echo "RPC:       $MONAD_RPC"
echo "ChainID:   $MONAD_CHAIN_ID"
echo "Explorer:  $MONAD_EXPLORER"
echo ""
echo "⚠️  请确保部署钱包已通过下方 faucet 领取测试币："
echo "    https://faucet.monad.xyz/"
echo ""

# ========================
# 1. 获取部署私钥
# ========================
echo "[1/6] 部署账户私钥（输入不回显）..."
read -s -p "  私钥 (0x...): " DEPLOYER_KEY
echo ""
if [[ ! "$DEPLOYER_KEY" =~ ^0x[a-fA-F0-9]{64}$ ]]; then
  echo "  ❌ 私钥格式错误，应为 0x + 64 位 hex"
  exit 1
fi

DEPLOYER_ADDR=$(cast wallet address "$DEPLOYER_KEY")
echo "  部署地址: $DEPLOYER_ADDR"
echo ""

# ========================
# 2. 检查余额
# ========================
echo "[2/6] 检查余额..."
BALANCE=$(cast balance "$DEPLOYER_ADDR" --rpc-url "$MONAD_RPC" 2>/dev/null || echo "0")
if [ "$BALANCE" = "0" ] || [ "$BALANCE" = "?" ]; then
  echo "  ❌ 余额为 0，请先领取测试币"
  unset DEPLOYER_KEY
  exit 1
fi
BALANCE_MON=$(echo "scale=4; $BALANCE / 10^18" | bc 2>/dev/null || echo "?")
echo "  余额: $BALANCE_MON MON"
if [ "$(echo "$BALANCE_MON < 0.5" | bc 2>/dev/null)" = "1" ]; then
  echo "  ⚠️  余额不足 0.5 MON，部署可能失败"
  read -p "  继续？(y/N): " CONT
  [ "$CONT" != "y" ] && unset DEPLOYER_KEY && exit 1
fi
echo ""

# ========================
# 3. 收集其他账户地址
# ========================
echo "[3/6] Evaluator 地址（持有 REGISTRY_EVALUATOR_ROLE / COMMERCE_EVALUATOR_ROLE / RESOLVER_ROLE）..."
read -p "  地址 (0x...)，留空使用部署账户: " EVALUATOR_ADDR
if [ -z "$EVALUATOR_ADDR" ]; then
  EVALUATOR_ADDR="$DEPLOYER_ADDR"
fi
echo "  Evaluator: $EVALUATOR_ADDR"

read -p "  Facilitator 地址（x402 支付，留空跳过）: " FACILITATOR_ADDR
echo ""

# ========================
# 4. 部署合约
# ========================
echo "[4/6] 部署合约到 Monad Testnet..."
echo "  MockERC20 → Registry → Hook → Job → 授权 → Seed 4 Agents"
echo ""

cd "$CONTRACTS"

# Deploy.s.sol 读取的环境变量
export EVALUATOR_ADDRESS="$EVALUATOR_ADDR"
export FACILITATOR_ADDRESS="${FACILITATOR_ADDR:-0x0000000000000000000000000000000000000000}"

FORGE_OUT=$(forge script script/Deploy.s.sol:Deploy \
  --rpc-url "$MONAD_RPC" \
  --broadcast \
  --slow \
  --private-key "$DEPLOYER_KEY" \
  --via-ir \
  --optimizer-runs 200 \
  2>&1) || {
    echo "  ❌ 部署失败，最后 30 行输出："
    echo "$FORGE_OUT" | tail -30
    unset DEPLOYER_KEY EVALUATOR_ADDRESS FACILITATOR_ADDRESS
    exit 1
  }

echo "$FORGE_OUT" | tail -20
echo ""

# 提取合约地址
TOKEN_ADDR=$(echo "$FORGE_OUT" | grep "MockERC20:" | grep -oE '0x[a-fA-F0-9]{40}' | tail -1)
REGISTRY_ADDR=$(echo "$FORGE_OUT" | grep "Registry:" | grep -oE '0x[a-fA-F0-9]{40}' | tail -1)
HOOK_ADDR=$(echo "$FORGE_OUT" | grep "ArbitrationHook:" | grep -oE '0x[a-fA-F0-9]{40}' | tail -1)
JOB_ADDR=$(echo "$FORGE_OUT" | grep "PrismSettleJob:" | grep -oE '0x[a-fA-F0-9]{40}' | tail -1)

if [ -z "$REGISTRY_ADDR" ] || [ -z "$JOB_ADDR" ] || [ -z "$HOOK_ADDR" ]; then
  echo "  ❌ 无法解析合约地址"
  echo "  请从上方输出中手动提取"
  unset DEPLOYER_KEY EVALUATOR_ADDRESS FACILITATOR_ADDRESS
  exit 1
fi

echo "  ✅ 部署成功！"
echo "  MockERC20:        $TOKEN_ADDR"
echo "  Registry:         $REGISTRY_ADDR"
echo "  ArbitrationHook:  $HOOK_ADDR"
echo "  PrismSettleJob:   $JOB_ADDR"
echo ""

# ========================
# 5. 回填 .env
# ========================
echo "[5/6] 更新 .env 的前端合约地址..."

# 用 python 做精准替换（避免 sed 跨平台问题）
python3 - "$ENV_FILE" "$REGISTRY_ADDR" "$JOB_ADDR" "$HOOK_ADDR" "$TOKEN_ADDR" <<'PYEOF'
import re, sys

env_path, reg, job, hook, token = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5]

with open(env_path, 'r') as f:
    content = f.read()

replacements = {
    'NEXT_PUBLIC_REGISTRY_ADDRESS=':     f'NEXT_PUBLIC_REGISTRY_ADDRESS={reg}',
    'NEXT_PUBLIC_JOB_CONTRACT_ADDRESS=':  f'NEXT_PUBLIC_JOB_CONTRACT_ADDRESS={job}',
    'NEXT_PUBLIC_HOOK_CONTRACT_ADDRESS=': f'NEXT_PUBLIC_HOOK_CONTRACT_ADDRESS={hook}',
    'NEXT_PUBLIC_PAYMENT_TOKEN_ADDRESS=': f'NEXT_PUBLIC_PAYMENT_TOKEN_ADDRESS={token}',
}

for old, new in replacements.items():
    # 只替换 key=XXX 的行，保留注释
    content = re.sub(
        rf'^{re.escape(old)}.*',
        new,
        content,
        flags=re.MULTILINE
    )

with open(env_path, 'w') as f:
    f.write(content)

print("  ✅ .env 合约地址已更新")
PYEOF

echo ""

# ========================
# 6. 写部署日志
# ========================
echo "[6/6] 写部署日志..."

mkdir -p "$ROOT/docs"
cat > "$DEPLOY_LOG" <<EOF
# Monad Testnet 部署日志

**部署时间：** $(date -u '+%Y-%m-%d %H:%M:%S UTC')
**部署账户：** $DEPLOYER_ADDR
**网络：** Monad Testnet (chainId=$MONAD_CHAIN_ID)
**RPC：** $MONAD_RPC
**Explorer：** $MONAD_EXPLORER

## 合约地址

| 合约 | 地址 | Explorer |
|------|------|----------|
| MockERC20 (测试 USDC) | $TOKEN_ADDR | [$MONAD_EXPLORER/address/$TOKEN_ADDR]($MONAD_EXPLORER/address/$TOKEN_ADDR) |
| PrismSettleRegistry | $REGISTRY_ADDR | [$MONAD_EXPLORER/address/$REGISTRY_ADDR]($MONAD_EXPLORER/address/$REGISTRY_ADDR) |
| ArbitrationHook | $HOOK_ADDR | [$MONAD_EXPLORER/address/$HOOK_ADDR]($MONAD_EXPLORER/address/$HOOK_ADDR) |
| PrismSettleJob | $JOB_ADDR | [$MONAD_EXPLORER/address/$JOB_ADDR]($MONAD_EXPLORER/address/$JOB_ADDR) |

## 角色配置

| 角色 | 持有人 |
|------|--------|
| REGISTRY_EVALUATOR_ROLE | $EVALUATOR_ADDR |
| COMMERCE_EVALUATOR_ROLE | $EVALUATOR_ADDR |
| RESOLVER_ROLE | $EVALUATOR_ADDR |

## 4 个官方 Agent

所有 4 个 Agent 已注册，初始声誉 0.7e18。
- DeFi Agent (0x1111)
- Labeling Agent (0x2222)
- Translate Agent (0x3333)
- Eval Agent (0x4444)

## 验证步骤

1. 启动 offchain:
   \`cd offchain && go run ./cmd -config config/prod.yaml\`

2. 前端加载：
   部署后 .env 已填入合约地址，docker compose 直接可用

3. 测试交易：
   \`\`\`bash
   cast call \$REGISTRY_ADDR "getScore(uint256)(uint256)" 0x1111 --rpc-url \$MONAD_RPC
   \`\`\`

## 完整部署日志

\`\`\`
$FORGE_OUT
\`\`\`
EOF

echo "  部署日志: $DEPLOY_LOG"
echo ""

# ========================
# 清理
# ========================
unset DEPLOYER_KEY EVALUATOR_ADDRESS FACILITATOR_ADDRESS

echo "============================================================"
echo "  ✅ 部署完成！"
echo "============================================================"
echo ""
echo "合约地址（已写入 .env）："
echo "  NEXT_PUBLIC_REGISTRY_ADDRESS=$REGISTRY_ADDR"
echo "  NEXT_PUBLIC_JOB_CONTRACT_ADDRESS=$JOB_ADDR"
echo "  NEXT_PUBLIC_HOOK_CONTRACT_ADDRESS=$HOOK_ADDR"
echo "  NEXT_PUBLIC_PAYMENT_TOKEN_ADDRESS=$TOKEN_ADDR"
echo ""
echo "下一步："
echo "  1. 打开 .env，填入 PRISM_EVALUATOR_KEY 和 PRISM_KEEPER_KEY"
echo "  2. 启动 docker-compose 验证全栈连通"
echo "  3. 在测试网上跑一笔 demo 验证"
