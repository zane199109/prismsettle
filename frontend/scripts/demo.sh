#!/bin/bash
# AI-assisted Dev Plan: PrismSettle Minimal Prototype
# One-command demo: deploy + full happy path on local Anvil
set -euo pipefail

cd "$(dirname "$0")/../../contracts"

unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY no_proxy NO_PROXY
export FOUNDRY_DISABLE_NIGHTLY_WARNING=1

# Anvil default accounts (from `anvil --help`)
BUYER_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
SELLER_KEY="0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
EVALUATOR_KEY="0x5de4111afa1a4b94908f8310ceb0f193495b573deef75f9520731291bc25ba0"
ADMIN_KEY="0x04e361e11e2d5371e2f4e0f4c9b0e5c2e6a8e2f1e3c4d5e6f7a8b9c0d1e2f3a4"

BUYER_ADDR=$(cast wallet address --private-key "$BUYER_KEY")
SELLER_ADDR=$(cast wallet address --private-key "$SELLER_KEY")
EVALUATOR_ADDR=$(cast wallet address --private-key "$EVALUATOR_KEY")

echo "=== Step 0: Start Anvil ==="
anvil --silent &
ANVIL_PID=$!
sleep 2

echo ""
echo "=== Step 1: Deploy contracts ==="
forge script script/Deploy.s.sol --rpc-url http://127.0.0.1:8545 --broadcast --legacy 2>&1 | tee /tmp/deploy.log

# Extract addresses from deploy log (format: "MockERC20: 0x...")
TOKEN_ADDR=$(grep 'MockERC20:' /tmp/deploy.log | grep -oE '0x[0-9a-fA-F]{40}')
REGISTRY_ADDR=$(grep 'Registry:' /tmp/deploy.log | grep -oE '0x[0-9a-fA-F]{40}')
JOB_ADDR=$(grep 'PrismSettleJob:' /tmp/deploy.log | grep -oE '0x[0-9a-fA-F]{40}')

echo ""
echo "Deployed:"
echo "  Token:      $TOKEN_ADDR"
echo "  Registry:   $REGISTRY_ADDR"
echo "  Job:        $JOB_ADDR"
echo "  Buyer:      $BUYER_ADDR"
echo "  Seller:     $SELLER_ADDR"
echo "  Evaluator:  $EVALUATOR_ADDR"

echo ""
echo "=== Step 2: Mint tokens + approve ==="
cast send --rpc-url http://127.0.0.1:8545 \
  --private-key "$BUYER_KEY" \
  "$TOKEN_ADDR" "mint(address,uint256)" "$BUYER_ADDR" "10000000000000000000000" 2>&1

cast send --rpc-url http://127.0.0.1:8545 \
  --private-key "$SELLER_KEY" \
  "$TOKEN_ADDR" "mint(address,uint256)" "$SELLER_ADDR" "10000000000000000000000" 2>&1

cast send --rpc-url http://127.0.0.1:8545 \
  --private-key "$BUYER_KEY" \
  "$TOKEN_ADDR" "approve(address,uint256)" "$JOB_ADDR" "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff" 2>&1

echo ""
echo "=== Step 3: Buyer creates job (agentId=0x1111, deadline=+1h) ==="
CREATE_TS=$(($(date +%s) + 3600))
CREATE_OUTPUT=$(cast send --rpc-url http://127.0.0.1:8545 \
  --private-key "$BUYER_KEY" \
  "$JOB_ADDR" "createJob(uint256,uint256,uint64,address)" \
  "0x1111" "0" "$CREATE_TS" "0x0000000000000000000000000000000000000000" 2>&1)
echo "$CREATE_OUTPUT"

JOB_ID=$(echo "$CREATE_OUTPUT" | grep -oE 'uint256: [0-9]+' | head -1 | grep -oE '[0-9]+')
echo "Created Job #${JOB_ID}"

echo ""
echo "=== Step 4: Fund job (10 USDC = 10e18 wei) ==="
cast send --rpc-url http://127.0.0.1:8545 \
  --private-key "$BUYER_KEY" \
  "$JOB_ADDR" "fundViaToken(uint256,uint256,bytes)" "$JOB_ID" "10000000000000000000" "" 2>&1
echo "Job funded with 10 USDC"

echo ""
echo "=== Step 5: Assign provider (seller) ==="
cast send --rpc-url http://127.0.0.1:8545 \
  --private-key "$BUYER_KEY" \
  "$JOB_ADDR" "assign(uint256,address)" "$JOB_ID" "$SELLER_ADDR" 2>&1
echo "Provider assigned"

echo ""
echo "=== Step 6: Provider submits proof ==="
PROOF_HASH=$(cast keccak "deliverable-proof-v1")
cast send --rpc-url http://127.0.0.1:8545 \
  --private-key "$SELLER_KEY" \
  "$JOB_ADDR" "submit(uint256,bytes32,bytes32)" "$JOB_ID" "$PROOF_HASH" "$PROOF_HASH" 2>&1
echo "Proof submitted: $PROOF_HASH"

echo ""
echo "=== Step 7: Evaluator completes job ==="
cast send --rpc-url http://127.0.0.1:8545 \
  --private-key "$EVALUATOR_KEY" \
  "$JOB_ADDR" "complete(uint256)" "$JOB_ID" 2>&1
echo "Job completed — escrow released!"

echo ""
echo "=== Step 8: Verify seller balance ==="
SELLER_BALANCE=$(cast call --rpc-url http://127.0.0.1:8545 \
  "$TOKEN_ADDR" "balanceOf(address)(uint256)" "$SELLER_ADDR" 2>&1)
echo "Seller USDC balance: $(echo "scale=2; $SELLER_BALANCE / 1000000000000000000" | bc) USDC"

echo ""
echo "=== Step 9: Check job state ==="
cast call --rpc-url http://127.0.0.1:8545 \
  "$JOB_ADDR" "getJobState(uint256)(uint8,address,address,uint256,bytes32,bytes32,uint64,address)" "$JOB_ID" 2>&1

echo ""
echo "=== DEMO COMPLETE ==="
echo "Full happy path: createJob → fund → assign → submit → complete"
kill $ANVIL_PID 2>/dev/null || true
