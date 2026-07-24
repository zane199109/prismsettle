#!/bin/bash
# PrismSettle — Monad Testnet Demo
# 验证完整流程：mint USDC → createJob → fund → assign → submit → complete
#
# 前置条件：
#   1. 已运行 deploy_monad_testnet.sh
#   2. .env 中有 DEPLOYER_KEY
#
# 用法：
#   bash scripts/demo-testnet.sh
#
# 注意：测试网上 deployer 同时担任 buyer 和 provider（因为 provider 需要签名 submit()）

set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT/.env"

# 从 .env 读取
RPC="https://testnet-rpc.monad.xyz"
TOKEN=$(grep "^NEXT_PUBLIC_PAYMENT_TOKEN_ADDRESS=" "$ENV_FILE" | cut -d= -f2)
JOB=$(grep "^NEXT_PUBLIC_JOB_CONTRACT_ADDRESS=" "$ENV_FILE" | cut -d= -f2)
REGISTRY=$(grep "^NEXT_PUBLIC_REGISTRY_ADDRESS=" "$ENV_FILE" | cut -d= -f2)
RAW_KEY=$(grep "^DEPLOYER_KEY=" "$ENV_FILE" | cut -d= -f2 | tr -d ' \t\n\r')
KEY="0x${RAW_KEY}"

BUYER=$(cast wallet address "$KEY")
AGENT="0x1111"
FUND_AMOUNT="10000000000000000000"  # 10 USDC

echo "=============================================="
echo "  PrismSettle — Monad Testnet Demo"
echo "=============================================="
echo ""
echo "RPC:            $RPC"
echo "Token:          $TOKEN"
echo "Registry:       $REGISTRY"
echo "Job:            $JOB"
echo "Buyer/Provider: $BUYER"
echo "Agent:          $AGENT"
echo ""

# 0. 获取当前 nonce
NONCE=$(cast call --rpc-url "$RPC" "$JOB" "buyerNonce(address)(uint256)" "$BUYER" 2>/dev/null | grep -oE '^[0-9]+')
echo "[0] Current buyerNonce: $NONCE"

# 1. Mint USDC
echo "[1] Mint 10000 USDC..."
cast send --rpc-url "$RPC" --private-key "$KEY" \
  "$TOKEN" "mint(address,uint256)" "$BUYER" "10000000000000000000000" >/dev/null 2>&1
echo "  ✅"

# 2. Approve
echo "[2] Approve Job contract..."
cast send --rpc-url "$RPC" --private-key "$KEY" \
  "$TOKEN" "approve(address,uint256)" "$JOB" "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff" >/dev/null 2>&1
echo "  ✅"

# 3. Create Job
echo "[3] Create Job..."
DEADLINE=$(($(date +%s) + 3600))
cast send --rpc-url "$RPC" --private-key "$KEY" \
  "$JOB" "createJob(uint256,uint256,uint64,address)" \
  "$AGENT" "0" "$DEADLINE" "0x0000000000000000000000000000000000000000" >/dev/null 2>&1

# JobId = uint256(keccak256(abi.encodePacked(buyer, nonce)))
PACKED="${BUYER:2}$(printf "%064x" $NONCE)"
JOB_ID=$(cast to-dec "$(cast keccak "0x$PACKED" 2>/dev/null | tail -1)" 2>/dev/null)
echo "  ✅ (jobId=$JOB_ID)"

# 4. Fund
echo "[4] Fund 10 USDC..."
cast send --rpc-url "$RPC" --private-key "$KEY" \
  "$JOB" "fundViaToken(uint256,uint256,bytes)" "$JOB_ID" "$FUND_AMOUNT" "0x" >/dev/null 2>&1
echo "  ✅"

# 5. Assign
echo "[5] Assign (provider=buyer)..."
cast send --rpc-url "$RPC" --private-key "$KEY" \
  "$JOB" "assign(uint256,address)" "$JOB_ID" "$BUYER" >/dev/null 2>&1
echo "  ✅"

# 6. Submit
echo "[6] Submit proof..."
PROOF_HASH=$(cast keccak "prism-demo-$(date +%s)" 2>/dev/null | tail -1)
cast send --rpc-url "$RPC" --private-key "$KEY" \
  "$JOB" "submit(uint256,bytes32,bytes32)" "$JOB_ID" "$PROOF_HASH" "$PROOF_HASH" >/dev/null 2>&1
echo "  ✅"

# 7. Complete
echo "[7] Complete..."
cast send --rpc-url "$RPC" --private-key "$KEY" \
  "$JOB" "complete(uint256)" "$JOB_ID" >/dev/null 2>&1
echo "  ✅"

# 8. Verify
echo ""
echo "=============================================="
echo "  Verify Results"
echo "=============================================="
echo ""
echo "  Job State (0=Created 4=Completed 5=Refunded):"
cast call --rpc-url "$RPC" "$JOB" "getJobState(uint256)(uint8,address,address,uint256,bytes32,bytes32,uint64,address)" "$JOB_ID" 2>/dev/null | head -1
echo ""
echo "  Buyer USDC Balance:"
cast call --rpc-url "$RPC" "$TOKEN" "balanceOf(address)(uint256)" "$BUYER" 2>/dev/null | head -1
echo ""
echo "  Agent 0x1111 Reputation:"
cast call --rpc-url "$RPC" "$REGISTRY" "getScore(uint256)(uint256)" "$AGENT" 2>/dev/null | head -1
echo ""
echo "=============================================="
echo "  ✅ DEMO COMPLETE"
echo "=============================================="
echo ""
echo "Explorer Links:"
echo "  Token:     https://testnet.monadexplorer.com/address/$TOKEN"
echo "  Job:       https://testnet.monadexplorer.com/address/$JOB"
echo "  Registry:  https://testnet.monadexplorer.com/address/$REGISTRY"
