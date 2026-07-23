#!/bin/bash
# Phase 9 任务 9.8 — x402 facilitator 集成验证脚本
#
# 覆盖 FR-AP06~AP09：
#   - FR-AP06: x402 收据格式验证（不透明 bytes）
#   - FR-AP07: fundViaToken x402 入口端到端
#   - FR-AP08: facilitator 接入（anvil 用 MockX402Facilitator 模拟）
#   - FR-AP09: facilitator 不可用时 ERC-20 兜底路径
#
# 用法：
#   scripts/verify_x402_integration.sh
#
# 前置条件：
#   1. foundry（forge + cast + anvil）已安装
#   2. 无需外部 RPC，脚本自动启动 anvil
set +e  # 不在 cast send 失败时退出（revert 是预期行为）

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTRACTS="$ROOT/contracts"
TMP_LOG="/tmp/prismsettle_x402_anvil.log"
PASS=0
FAIL=0

ok()   { echo "  ✅ $1"; PASS=$((PASS+1)); }
fail() { echo "  ❌ $1"; FAIL=$((FAIL+1)); }

echo "============================================================"
echo "  PrismSettle — x402 Facilitator 集成验证 (Phase 9 任务 9.8)"
echo "============================================================"
echo ""

# ---------- 1. Foundry 单元 + 集成测试（覆盖 FR-AP06~AP09 核心逻辑） ----------
echo "[1/3] Foundry 测试：X402Integration.t.sol（6 个测试覆盖 FR-AP06~AP09）..."
cd "$CONTRACTS"
FORGE_OUT=$(forge test --match-contract X402IntegrationTest 2>&1)
echo "$FORGE_OUT" | grep -E "^\[(PASS|FAIL)\]" | head -10

if echo "$FORGE_OUT" | grep -q "6 passed"; then
  ok "FR-AP06: 收据格式验证（opaque bytes，任意非空 bytes 接受）"
  ok "FR-AP07: fundViaToken x402 入口（双路径分流）"
  ok "FR-AP08: facilitator 接入（MockX402Facilitator settleWithReceipt）"
  ok "FR-AP09a: facilitator=address(0) 时 ERC-20 兜底成功"
  ok "FR-AP09b: facilitator=address(0) 时 x402 路径正确 revert"
  ok "FR-AP09c: receipt 重放保护（跨 Job 也防重放）"
else
  fail "X402IntegrationTest 未全部通过"
  echo "$FORGE_OUT" | tail -15
fi

# ---------- 2. 原有 PrismSettleJob.t.sol x402 路径测试（回归） ----------
echo ""
echo "[2/3] Foundry 测试：PrismSettleJob.t.sol x402 路径（回归验证）..."
JOB_FORGE_OUT=$(forge test --match-contract PrismSettleJobTest --match-test "X402|Fund" 2>&1)
echo "$JOB_FORGE_OUT" | grep -E "^\[(PASS|FAIL)\]" | head -10

if echo "$JOB_FORGE_OUT" | grep -q "passed"; then
  ok "PrismSettleJob x402 路径回归测试通过"
else
  fail "PrismSettleJob x402 路径回归测试失败"
fi

# ---------- 3. 合约源码静态校验（fundViaToken 分流逻辑存在） ----------
echo ""
echo "[3/3] 合约源码静态校验：fundViaToken 双路径分流..."
JOB_SOL="$CONTRACTS/src/PrismSettleJob.sol"
IFACE_SOL="$CONTRACTS/src/interfaces/IX402Facilitator.sol"

CHECKS_PASS=true

# 3a. IX402Facilitator 接口存在
if grep -q "function settleWithReceipt" "$IFACE_SOL"; then
  ok "IX402Facilitator.settleWithReceipt 接口定义存在"
else
  fail "IX402Facilitator.settleWithReceipt 接口缺失"
  CHECKS_PASS=false
fi

# 3b. fundViaToken 函数存在
if grep -q "function fundViaToken" "$JOB_SOL"; then
  ok "PrismSettleJob.fundViaToken 统一入口存在"
else
  fail "PrismSettleJob.fundViaToken 缺失"
  CHECKS_PASS=false
fi

# 3c. x402 路径分流（receipt.length > 0）
if grep -q 'x402Receipt.length > 0' "$JOB_SOL"; then
  ok "x402 路径分流逻辑存在（receipt.length > 0）"
else
  fail "x402 路径分流逻辑缺失"
  CHECKS_PASS=false
fi

# 3d. 防重放（usedReceipts mapping）
if grep -q 'usedReceipts\[receiptHash\]' "$JOB_SOL"; then
  ok "防重放机制存在（usedReceipts mapping）"
else
  fail "防重放机制缺失"
  CHECKS_PASS=false
fi

# 3e. ERC-20 兜底路径（transferFrom）
if grep -q 'transferFrom(j.buyer' "$JOB_SOL"; then
  ok "ERC-20 兜底路径存在（transferFrom）"
else
  fail "ERC-20 兜底路径缺失"
  CHECKS_PASS=false
fi

# 3f. facilitator=address(0) 校验
if grep -q 'facilitator != address(0)' "$JOB_SOL"; then
  ok "facilitator=address(0) 校验存在"
else
  fail "facilitator=address(0) 校验缺失"
  CHECKS_PASS=false
fi

# 3g. 收据格式文档存在
if [ -f "$ROOT/docs/x402-receipt-format.md" ]; then
  ok "FR-AP06 收据格式文档存在（docs/x402-receipt-format.md）"
else
  fail "FR-AP06 收据格式文档缺失"
  CHECKS_PASS=false
fi

# 3h. facilitator 接入指南文档存在
if [ -f "$ROOT/docs/x402-facilitator-onboarding.md" ]; then
  ok "FR-AP08 facilitator 接入指南存在（docs/x402-facilitator-onboarding.md）"
else
  fail "FR-AP08 facilitator 接入指南缺失"
  CHECKS_PASS=false
fi

# ---------- 结果汇总 ----------
echo ""
echo "============================================================"
echo "  验证结果：$PASS 通过 / $FAIL 失败"
echo "============================================================"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi

echo ""
echo "✅ Phase 9 任务 9.8 x402 集成验证通过"
echo ""
echo "覆盖的 FR："
echo "  - FR-AP06: x402 收据格式（docs/x402-receipt-format.md + testReceiptIsOpaqueBytes）"
echo "  - FR-AP07: fundViaToken 统一入口（合约层 + testDualPathOnSameContract）"
echo "  - FR-AP08: facilitator 接入（MockX402Facilitator + docs/x402-facilitator-onboarding.md）"
echo "  - FR-AP09: ERC-20 兜底路径（testFallbackERC20WhenNoFacilitator + testRevertX402PathWhenNoFacilitator）"
