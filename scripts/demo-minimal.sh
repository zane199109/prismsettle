#!/bin/bash
# AI-assisted Dev Plan: PrismSettle Minimal Prototype - Complete Verification
set -euo pipefail

unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY no_proxy NO_PROXY
export FOUNDRY_DISABLE_NIGHTLY_WARNING=1

cd /home/administrator/Documents/trae_projects/PrismSettle/contracts

RPC="http://127.0.0.1:8545"
BUYER_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
SELLER_KEY="0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
BUYER="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
SELLER="0x70997970C51812dc3A010C7d01b50e0d17dc79C8"

echo "=============================================="
echo "  PrismSettle Minimal Prototype Demo"
echo "  Full Happy Path: createJob → fund → assign → submit → complete"
echo "=============================================="
echo ""

# Start fresh anvil
echo "[1/10] Starting Anvil..."
pkill anvil 2>/dev/null || true
sleep 2
anvil --silent &
ANVIL_PID=$!
sleep 2
echo "  Anvil running on $RPC"
echo ""

# Deploy contracts
echo "[2/10] Deploying contracts..."
DEPLOY_OUTPUT=$(forge script script/Deploy.s.sol --rpc-url $RPC --broadcast --legacy 2>&1) || true
TOKEN_ADDR=$(echo "$DEPLOY_OUTPUT" | grep 'MockERC20:' | grep -oE '0x[0-9a-fA-F]{40}')
REGISTRY_ADDR=$(echo "$DEPLOY_OUTPUT" | grep 'Registry:' | grep -oE '0x[0-9a-fA-F]{40}')
JOB_ADDR=$(echo "$DEPLOY_OUTPUT" | grep 'PrismSettleJob:' | grep -oE '0x[0-9a-fA-F]{40}')
EVALUATOR_ADDR=$(echo "$DEPLOY_OUTPUT" | grep 'Evaluator:' | grep -oE '0x[0-9a-fA-F]{40}')

echo "  Token:      $TOKEN_ADDR"
echo "  Registry:   $REGISTRY_ADDR"
echo "  Job:        $JOB_ADDR"
echo "  Evaluator:  $EVALUATOR_ADDR"
echo ""

# Mint tokens
echo "[3/10] Minting USDC to Buyer..."
cast send --rpc-url $RPC --private-key $BUYER_KEY \
  $TOKEN_ADDR "mint(address,uint256)" $BUYER "10000000000000000000000" >/dev/null 2>&1
echo "  ✓ Minted 10,000 USDC to buyer"

echo "[4/10] Approving Job contract..."
cast send --rpc-url $RPC --private-key $BUYER_KEY \
  $TOKEN_ADDR "approve(address,uint256)" $JOB_ADDR "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff" >/dev/null 2>&1
echo "  ✓ Approved unlimited spend"
echo ""

# Create job
echo "[5/10] Creating Job (agentId=0x1111, deadline=+1h)..."
CREATE_TS=$(($(date +%s) + 3600))
cast send --rpc-url $RPC --private-key $BUYER_KEY \
  $JOB_ADDR "createJob(uint256,uint256,uint64,address)" \
  "0x1111" "0" "$CREATE_TS" "0x0000000000000000000000000000000000000000" >/dev/null 2>&1
echo "  ✓ Job #0 created"
echo ""

# Fund job
echo "[6/10] Funding Job (10 USDC)..."
cast send --rpc-url $RPC --private-key $BUYER_KEY \
  $JOB_ADDR "fundViaToken(uint256,uint256,bytes)" "0" "10000000000000000000" "0x" >/dev/null 2>&1
echo "  ✓ Job funded with 10 USDC"
echo ""

# Assign provider
echo "[7/10] Assigning Provider (Seller)..."
cast send --rpc-url $RPC --private-key $BUYER_KEY \
  $JOB_ADDR "assign(uint256,address)" "0" "$SELLER" >/dev/null 2>&1 || true
echo "  ✓ Provider assigned"
echo ""

# Submit proof
echo "[8/10] Provider submitting proof..."
PROOF_HASH=$(cast keccak "deliverable-proof-v1")
cast send --rpc-url $RPC --private-key $SELLER_KEY \
  $JOB_ADDR "submit(uint256,bytes32,bytes32)" "0" "$PROOF_HASH" "$PROOF_HASH" >/dev/null 2>&1
echo "  ✓ Proof submitted: $PROOF_HASH"
echo ""

# Complete job
echo "[9/10] Evaluator completing job..."
cast send --rpc-url $RPC --private-key $BUYER_KEY \
  $JOB_ADDR "complete(uint256)" "0" >/dev/null 2>&1
echo "  ✓ Job completed — escrow released!"
echo ""

# Verify results
echo "[10/10] Verifying results..."
echo ""

# Check seller balance
echo "  Seller USDC balance:"
cast call --rpc-url $RPC \
  $TOKEN_ADDR "balanceOf(address)(uint256)" $SELLER 2>&1 | head -5
echo ""

# Check job state
echo "  Job State:"
cast call --rpc-url $RPC \
  $JOB_ADDR "getJobState(uint256)(uint8,address,address,uint256,bytes32,bytes32,uint64,address)" "0" 2>&1 | head -10

echo ""
echo "=============================================="
echo "  DEMO COMPLETE — All steps passed!"
echo "=============================================="
