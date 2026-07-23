#!/bin/bash
# Phase 9 任务 9.7 — Monad Testnet 部署脚本
#
# ⚠️  安全须知：
#   - 私钥通过 `read -s` 隐藏输入，绝不写入磁盘或日志
#   - 部署完毕后立即 `unset` 所有私钥环境变量
#   - 请确保本机无恶意软件（keylogger 等）
#   - 推荐使用专用测试钱包，切勿使用主网钱包
#
# 前置条件：
#   1. 已安装 foundry (forge, cast)
#   2. 已通过 Monad faucet 领取测试网 MON（每个账户 ≥ 1 MON）
#      Faucet: https://faucet.monad.xyz/
#   3. 部署账户、Evaluator 账户、Keeper 账户均已领币
#
# 用法：
#   scripts/deploy_monad_testnet.sh
#
# 部署成功后，脚本会自动：
#   - 输出 4 个合约地址
#   - 更新 offchain/config/prod.yaml 的 contract_addr
#   - 生成 docs/testnet-deploy-log.md 记录部署信息
set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTRACTS="$ROOT/contracts"
CONFIG="$ROOT/offchain/config/prod.yaml"
DEPLOY_LOG="$ROOT/docs/testnet-deploy-log.md"

# Monad Testnet 配置（DEV-PLAN §9.7 + PRD §8.3）
MONAD_RPC="https://testnet-rpc.monad.xyz"
MONAD_CHAIN_ID=10143
MONAD_EXPLORER="https://testnet.monadexplorer.com"

echo "============================================================"
echo "  PrismSettle — Monad Testnet 部署 (Phase 9 任务 9.7)"
echo "============================================================"
echo ""
echo "RPC:       $MONAD_RPC"
echo "ChainID:   $MONAD_CHAIN_ID"
echo "Explorer:  $MONAD_EXPLORER"
echo ""
echo "⚠️  请确保已通过 https://faucet.monad.xyz/ 领取测试币"
echo "    部署账户 + Evaluator + Keeper 均需 ≥ 1 MON"
echo ""

# ---------- 1. 收集私钥（隐藏输入，不写入文件） ----------

echo "[1/6] 收集部署账户私钥（输入不回显）..."
echo "    部署账户将作为合约 admin，持有 REGISTRY_EVALUATOR_ROLE"
read -s -p "  部署账户私钥 (0x...): " DEPLOYER_KEY
echo ""
if [[ ! "$DEPLOYER_KEY" =~ ^0x[a-fA-F0-9]{64}$ ]]; then
  echo "  ❌ 私钥格式错误（应为 0x + 64 位 hex）"
  exit 1
fi
export DEPLOYER_KEY

DEPLOYER_ADDR=$(cast wallet address "$DEPLOYER_KEY")
echo "  部署账户地址: $DEPLOYER_ADDR"

echo ""
echo "[2/6] 检查部署账户余额..."
BALANCE=$(cast balance "$DEPLOYER_ADDR" --rpc-url "$MONAD_RPC" 2>/dev/null || echo "0")
BALANCE_MON=$(echo "scale=4; $BALANCE / 10^18" | bc 2>/dev/null || echo "?")
echo "  余额: $BALANCE_MON MON ($BALANCE wei)"

if [ "$BALANCE" = "0" ] || [ "$BALANCE" = "?" ]; then
  echo "  ❌ 余额为 0，请先到 https://faucet.monad.xyz/ 领取测试币"
  unset DEPLOYER_KEY
  exit 1
fi
if [ "$(echo "$BALANCE_MON < 1.0" | bc)" = "1" ]; then
  echo "  ⚠️  余额不足 1 MON，部署可能失败，建议再领币"
  read -p "  仍然继续？(y/N): " CONT
  [ "$CONT" != "y" ] && unset DEPLOYER_KEY && exit 1
fi

echo ""
echo "[3/6] 收集 Evaluator 账户（用于 submitValidation 签名）..."
echo "    Evaluator 持有 REGISTRY_EVALUATOR_ROLE / COMMERCE_EVALUATOR_ROLE / RESOLVER_ROLE"
read -p "  Evaluator 地址 (0x...)，留空则使用部署账户: " EVALUATOR_ADDR
if [ -z "$EVALUATOR_ADDR" ]; then
  EVALUATOR_ADDR="$DEPLOYER_ADDR"
  echo "  使用部署账户作为 Evaluator: $EVALUATOR_ADDR"
fi

echo ""
echo "  （可选）Facilitator 地址（x402 支付，留空则 address(0)，后续可设置）"
read -p "  Facilitator 地址 (0x...)，留空跳过: " FACILITATOR_ADDR

# ---------- 4. 部署合约 ----------

echo ""
echo "[4/6] 部署三合约到 Monad Testnet..."
echo "    MockERC20 → Registry → ArbitrationHook → PrismSettleJob → 角色授权 → Seed 4 Agents"
echo ""

cd "$CONTRACTS"
EXPORT_EVALUATOR="$EVALUATOR_ADDR"
EXPORT_FACILITATOR="${FACILITATOR_ADDR:-0x0000000000000000000000000000000000000000}"

# 使用 --slow 单笔确认模式，避免 nonce 卡住（DEV-PLAN §9.7 重试策略）
FORGE_OUT=$(forge script script/Deploy.s.sol:Deploy \
  --rpc-url "$MONAD_RPC" \
  --broadcast \
  --slow \
  --private-key "$DEPLOYER_KEY" \
  --via-ir \
  --verify \
  --chain-id "$MONAD_CHAIN_ID" \
  --optimizer-runs 200 \
  2>&1) || {
    echo "  ❌ forge script 失败，最后 30 行输出："
    echo "$FORGE_OUT" | tail -30
    unset DEPLOYER_KEY
    exit 1
  }

echo "$FORGE_OUT" | tail -50

# 从 forge 输出中提取合约地址
TOKEN_ADDR=$(echo "$FORGE_OUT" | grep "MockERC20:" | awk '{print $NF}' | tail -1)
REGISTRY_ADDR=$(echo "$FORGE_OUT" | grep "Registry:" | awk '{print $NF}' | tail -1)
HOOK_ADDR=$(echo "$FORGE_OUT" | grep "ArbitrationHook:" | awk '{print $NF}' | tail -1)
JOB_ADDR=$(echo "$FORGE_OUT" | grep "PrismSettleJob:" | awk '{print $NF}' | tail -1)

if [ -z "$REGISTRY_ADDR" ] || [ -z "$JOB_ADDR" ] || [ -z "$HOOK_ADDR" ]; then
  echo ""
  echo "  ❌ 无法从 forge 输出解析合约地址"
  echo "  请手动从上方输出提取地址，并填入 $CONFIG"
  unset DEPLOYER_KEY
  exit 1
fi

echo ""
echo "  ✅ 部署成功！"
echo "  MockERC20:        $TOKEN_ADDR"
echo "  PrismSettleRegistry: $REGISTRY_ADDR"
echo "  ArbitrationHook:  $HOOK_ADDR"
echo "  PrismSettleJob:   $JOB_ADDR"

# ---------- 5. 更新 prod.yaml ----------

echo ""
echo "[5/6] 更新 offchain/config/prod.yaml 的合约地址..."

# 用 sed 替换 prismsettle_registry / job / hook 三处 contract_addr
# 仅替换 # replace after deploy 的占位行
python3 - "$CONFIG" "$REGISTRY_ADDR" "$JOB_ADDR" "$HOOK_ADDR" <<'PY'
import re, sys
cfg, reg, job, hook = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
with open(cfg, 'r') as f:
    content = f.read()
# 仅替换 contract_addr: "0x0000..." # replace after deploy 行
content = re.sub(
    r'(chain_name:\s*"prismsettle_registry"[\s\S]*?contract_addr:\s*)"[^"]*"\s*#\s*replace after deploy',
    rf'\1"{reg}"  # auto-updated by deploy_monad_testnet.sh',
    content, count=1, flags=re.MULTILINE)
content = re.sub(
    r'(chain_name:\s*"prismsettle_job"[\s\S]*?contract_addr:\s*)"[^"]*"\s*#\s*replace after deploy',
    rf'\1"{job}"  # auto-updated by deploy_monad_testnet.sh',
    content, count=1, flags=re.MULTILINE)
content = re.sub(
    r'(chain_name:\s*"prismsettle_hook"[\s\S]*?contract_addr:\s*)"[^"]*"\s*#\s*replace after deploy',
    rf'\1"{hook}"  # auto-updated by deploy_monad_testnet.sh',
    content, count=1, flags=re.MULTILINE)
# 同时更新 evaluator_address
content = re.sub(
    r'(evaluator_address:\s*)"[^"]*"',
    rf'\1"{sys.argv[5] if len(sys.argv)>5 else ""}"',
    content, count=1)
with open(cfg, 'w') as f:
    f.write(content)
print("  ✅ prod.yaml 已更新")
PY

# 重新跑一次带上 evaluator_address
python3 - "$CONFIG" "$EVALUATOR_ADDR" <<'PY'
import re, sys
cfg, eval_addr = sys.argv[1], sys.argv[2]
with open(cfg, 'r') as f:
    content = f.read()
content = re.sub(
    r'(evaluator_address:\s*)"[^"]*"',
    rf'\1"{eval_addr}"  # auto-updated by deploy_monad_testnet.sh',
    content, count=1)
with open(cfg, 'w') as f:
    f.write(content)
print(f"  ✅ evaluator_address 已更新为 {eval_addr}")
PY

# ---------- 6. 写部署日志 ----------

echo ""
echo "[6/6] 写部署日志 docs/testnet-deploy-log.md..."
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

| 角色 | 持有人 | 说明 |
|------|--------|------|
| REGISTRY_EVALUATOR_ROLE | $EVALUATOR_ADDR | submitValidation 签名 |
| COMMERCE_EVALUATOR_ROLE | $EVALUATOR_ADDR | complete/abort Job |
| RESOLVER_ROLE | $EVALUATOR_ADDR | ArbitrationHook 仲裁 |
| Facilitator | ${FACILITATOR_ADDR:-未设置（x402 兜底）} | x402 支付验证 |

## 4 个官方 Agent（Seed Phase，PRD FR-C09）

| Agent | agentId | endpointUrl |
|-------|---------|-------------|
| DeFi Agent | 0x1111 | http://localhost:8001/invoke |
| Labeling Agent | 0x2222 | http://localhost:8002/invoke |
| Translate Agent | 0x3333 | http://localhost:8003/invoke |
| Eval Agent | 0x4444 | http://localhost:8004/invoke |

**初始声誉分：** 0.7e18 (0.7) — 4 个 Agent 均已 seed

## 后续验证步骤

1. **Indexer 连接 testnet：**
   \`\`\`
   cd offchain && go run ./cmd -config config/prod.yaml
   \`\`\`
   检查日志：sync_lag < 5 blocks，无 RPC 错误

2. **/health 端点：**
   \`\`\`
   curl http://localhost:9527/health
   \`\`\`
   预期 status=ok 或 degraded（evaluator_state=stopped 时 degraded）

3. **前端 constants.ts 更新：**
   将以下地址填入前端合约配置：
   - REGISTRY_ADDR = "$REGISTRY_ADDR"
   - JOB_ADDR = "$JOB_ADDR"
   - HOOK_ADDR = "$HOOK_ADDR"
   - TOKEN_ADDR = "$TOKEN_ADDR"
   - CHAIN_ID = $MONAD_CHAIN_ID

4. **Evaluator 测试：**
   - 配置 offchain/config/prod.yaml evaluator.private_key
   - 启动 evaluator，调用 complete + submitValidation
   - 检查 testnet 上事件确认

5. **Agent 公网部署：**
   - 4 个 Agent endpointUrl 需公网可达
   - 更新 agent_registry 的 metadata.endpointUrl
   - 用 cast send 调用 registry.updateAgentMeta(agentId, newMeta)

## 部署输出（forge script 完整日志）

\`\`\`
$FORGE_OUT
\`\`\`
EOF

# ---------- 清理敏感环境变量 ----------

unset DEPLOYER_KEY
echo ""
echo "============================================================"
echo "  ✅ Phase 9 任务 9.7 部署完成"
echo "============================================================"
echo ""
echo "  合约地址："
echo "    Registry: $REGISTRY_ADDR"
echo "    Job:      $JOB_ADDR"
echo "    Hook:     $HOOK_ADDR"
echo "    Token:    $TOKEN_ADDR"
echo ""
echo "  配置文件：$CONFIG 已自动更新"
echo "  部署日志：$DEPLOY_LOG"
echo ""
echo "  下一步："
echo "    1. 启动 indexer 连接 testnet（cd offchain && go run ./cmd -config config/prod.yaml）"
echo "    2. 把合约地址告诉我（或直接粘贴 $DEPLOY_LOG 内容），我帮你更新前端 constants"
echo "    3. 配置 evaluator.private_key（在 $CONFIG 的 evaluator 段）"
echo "    4. 启动 4 个 Agent 服务"
echo ""
echo "  ⚠️  部署账户私钥已从环境变量清除"
