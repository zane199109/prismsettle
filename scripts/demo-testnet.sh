#!/usr/bin/env bash
# ============================================================================
# PrismSettle Demo — Monad Testnet
# ============================================================================
# One-click script that walks through the full settlement lifecycle:
#   create → fund → grabJob → submit → complete (buyer + rating score)
#
# Usage:
#   bash scripts/demo-testnet.sh
#
# Prerequisites:
#   - foundry (cast)
#   - jq
#   - .env with DEPLOYER_KEY / BUYER_KEY / PROVIDER_KEY (fallback: env vars)
# ============================================================================

set -euo pipefail

# ── Colors ──────────────────────────────────────────────────────────────────
BOLD='\033[1m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info()  { echo -e "${CYAN}◆${NC} $*"; }
ok()    { echo -e "${GREEN}✓${NC} $*"; }
warn()  { echo -e "${YELLOW}⚠${NC} $*"; }
header(){ echo -e "\n${BOLD}━━━ $* ━━━${NC}"; }

# ── Config ───────────────────────────────────────────────────────────────────
RPC="https://testnet-rpc.monad.xyz"
TOKEN="0x252e44550f8B9997901e5540FC0E1dA52Ab099C6"
REGISTRY="0xA82937ad81e8aB775c9B32F363CE5E8564207739"
JOB="0x4B09DB038dF842277f3f1aD4500b9BEDBFcB47cB"
HOOK="0x740c2969e537706A4f4757166e5eBEeD0E4DAD15"
FUND_AMOUNT=100000000000000000000  # 100 USDC (18 decimals)

# Deployer key: try env var, then .env
DEPLOYER_KEY="${DEPLOYER_KEY:-}"
if [ -z "$DEPLOYER_KEY" ] && [ -f .env ]; then
  DEPLOYER_KEY=$(grep '^DEPLOYER_KEY=' .env | head -1 | cut -d= -f2 | tr -d ' \n')
fi
if [ -z "$DEPLOYER_KEY" ]; then
  echo "ERROR: DEPLOYER_KEY not set. Export it or add to .env"
  exit 1
fi

# Derive addresses
DEPLOYER=$(cast wallet address --private-key "$DEPLOYER_KEY")

# ── 1. Header ────────────────────────────────────────────────────────────────
header "PrismSettle Demo — Monad Testnet"
echo ""
echo "  RPC:       $RPC"
echo "  Deployer:  $DEPLOYER"
echo "  Token:     $TOKEN"
echo "  Job:       $JOB"
echo "  Hook:      $HOOK"
echo ""

# Check balances
DEP_BAL=$(cast balance --rpc-url "$RPC" "$DEPLOYER" 2>/dev/null)
echo "  Deployer balance:  $(echo "scale=4; $DEP_BAL / 10^18" | bc) MON"
echo ""

# ── 2. Buyer wallet ──────────────────────────────────────────────────────────
header "Step 1: Buyer Wallet"
BUYER_KEY="${BUYER_KEY:-}"
if [ -z "$BUYER_KEY" ] && [ -f .env ]; then
  BUYER_KEY=$(grep '^BUYER_KEY=' .env | head -1 | cut -d= -f2 | tr -d ' \n')
fi
if [ -z "$BUYER_KEY" ]; then
  warn "BUYER_KEY not set in .env — generating a temporary wallet"
  BUYER_KEY=$(cast wallet new --json | jq -r '.[0].private_key')
fi
BUYER=$(cast wallet address --private-key "$BUYER_KEY")
echo "  Buyer: $BUYER"
echo ""

# ── 3. Fund buyer with MON ───────────────────────────────────────────────────
header "Step 2: Fund Buyer with MON"
info "Sending 0.1 MON to buyer for gas..."
TX=$(cast send --rpc-url "$RPC" --private-key "$DEPLOYER_KEY" \
  --value 0.1ether "$BUYER" --json)
TX_HASH=$(echo "$TX" | jq -r '.transactionHash')
ok "Sent 0.1 MON → $BUYER  tx=$TX_HASH"
echo ""

# ── 4. Mint USDC to buyer ────────────────────────────────────────────────────
header "Step 3: Mint USDC to Buyer"
info "Minting 1,000 USDC..."
TX=$(cast send --rpc-url "$RPC" --private-key "$DEPLOYER_KEY" \
  "$TOKEN" "mint(address,uint256)" "$BUYER" 1000000000000000000000 --json)
TX_HASH=$(echo "$TX" | jq -r '.transactionHash')
ok "Minted 1,000 USDC → $BUYER  tx=$TX_HASH"

# Verify balance
BAL=$(cast call --rpc-url "$RPC" "$TOKEN" "balanceOf(address)(uint256)" "$BUYER")
echo "  Buyer USDC balance: $(echo "scale=2; $BAL / 10^18" | bc) USDC"
echo ""

# ── 5. Provider wallet ───────────────────────────────────────────────────────
header "Step 4: Provider Wallet"
PROVIDER_KEY="${PROVIDER_KEY:-}"
if [ -z "$PROVIDER_KEY" ] && [ -f .env ]; then
  PROVIDER_KEY=$(grep '^PROVIDER_KEY=' .env | head -1 | cut -d= -f2 | tr -d ' \n')
fi
if [ -z "$PROVIDER_KEY" ]; then
  warn "PROVIDER_KEY not set in .env — generating a temporary wallet"
  PROVIDER_KEY=$(cast wallet new --json | jq -r '.[0].private_key')
fi
PROVIDER=$(cast wallet address --private-key "$PROVIDER_KEY")
echo "  Provider: $PROVIDER"
echo ""

# ── 6. Fund provider with MON ─────────────────────────────────────────────────
header "Step 5: Fund Provider with MON"
info "Sending 0.05 MON to provider for gas..."
TX=$(cast send --rpc-url "$RPC" --private-key "$DEPLOYER_KEY" \
  --value 0.05ether "$PROVIDER" --json)
TX_HASH=$(echo "$TX" | jq -r '.transactionHash')
ok "Sent 0.05 MON → $PROVIDER  tx=$TX_HASH"
echo ""

# ── 7. Approve job contract ──────────────────────────────────────────────────
header "Step 6: Approve Job Contract"
info "Approving $JOB to spend USDC..."
TX=$(cast send --rpc-url "$RPC" --private-key "$BUYER_KEY" \
  "$TOKEN" "approve(address,uint256)" "$JOB" $FUND_AMOUNT --json)
TX_HASH=$(echo "$TX" | jq -r '.transactionHash')
ok "Approved  tx=$TX_HASH"
echo ""

# ── 8. Create job ────────────────────────────────────────────────────────────
header "Step 7: Create Job"
DEADLINE=$(( $(date +%s) + 3600 ))  # 1 hour from now
info "Creating job (agentId=0x1111, deadline=$DEADLINE, hook=$HOOK)..."
TX=$(cast send --rpc-url "$RPC" --private-key "$BUYER_KEY" \
  "$JOB" "createJob(uint256,uint256,uint64,address,uint96,address)" \
  0x1111 0 "$DEADLINE" "$HOOK" 0 0x0000000000000000000000000000000000000000 --json)
TX_HASH=$(echo "$TX" | jq -r '.transactionHash')
# Extract jobId from JobCreated event (topic[2] = indexed jobId)
JOB_ID=$(cast receipt --rpc-url "$RPC" "$TX_HASH" --json | jq -r '.logs[0].topics[2]')
echo "  Job ID: 0x${JOB_ID#0x} (tx=$TX_HASH)"
ok "Job created"
echo ""

# ── 9. Fund job ──────────────────────────────────────────────────────────────
header "Step 8: Fund Job (Escrow)"
info "Depositing 100 USDC into escrow..."
TX=$(cast send --rpc-url "$RPC" --private-key "$BUYER_KEY" \
  "$JOB" "fundViaToken(uint256,uint256,bytes)" \
  "$JOB_ID" $FUND_AMOUNT "0x" --json)
TX_HASH=$(echo "$TX" | jq -r '.transactionHash')
ok "Funded 100 USDC  tx=$TX_HASH"

# Verify job state
STATE=$(cast call --rpc-url "$RPC" "$JOB" "getJobState(uint256)" "$JOB_ID" 2>/dev/null | head -1)
echo "  Job state: $STATE"
echo ""

# ── 10. Grab job ────────────────────────────────────────────────────────────
header "Step 9: Grab Job (Provider 0x5555)"
# 注意：0x5555 已在部署时注册并 seed 0.7e18（Deploy.s.sol 第 8 步），无需重复注册
info "Provider grabbing job (reputation 0.7e18)..."
TX=$(cast send --rpc-url "$RPC" --private-key "$PROVIDER_KEY" \
  "$JOB" "grabJob(uint256,uint256)" "$JOB_ID" "0x5555" --json)
TX_HASH=$(echo "$TX" | jq -r '.transactionHash')
ok "Grabbed  tx=$TX_HASH"
echo ""

# ── 11. Submit proof (Provider signs) ─────────────────────────────────────────
header "Step 10: Submit Work (Provider)"
DELIVERABLE_HASH=$(cast keccak "deliverable-v1-data")
PROOF_HASH=$(cast keccak "execution-proof-v1")
info "Submitting deliverableHash=$DELIVERABLE_HASH"
TX=$(cast send --rpc-url "$RPC" --private-key "$PROVIDER_KEY" \
  "$JOB" "submit(uint256,bytes32,bytes32)" \
  "$JOB_ID" "$DELIVERABLE_HASH" "$PROOF_HASH" --json)
TX_HASH=$(echo "$TX" | jq -r '.transactionHash')
ok "Submitted  tx=$TX_HASH"
echo ""

# ── 12. Complete (Buyer + rating) ───────────────────────────────────────────
header "Step 11: Buyer Completes Job + Rating"
info "Buyer completing job with rating 0.9e18 (score=900000000000000000)..."
TX=$(cast send --rpc-url "$RPC" --private-key "$BUYER_KEY" \
  "$JOB" "complete(uint256,uint96)" "$JOB_ID" "900000000000000000" --json)
TX_HASH=$(echo "$TX" | jq -r '.transactionHash')
ok "Completed  tx=$TX_HASH"

# Verify final state
STATE=$(cast call --rpc-url "$RPC" "$JOB" "getJobState(uint256)" "$JOB_ID" 2>/dev/null | head -1)
echo "  Job state: $STATE"
echo ""

# ── 13. Show results ─────────────────────────────────────────────────────────
header "Result"
PROV_BAL=$(cast call --rpc-url "$RPC" "$TOKEN" "balanceOf(address)(uint256)" "$PROVIDER")
BUY_BAL=$(cast call --rpc-url "$RPC" "$TOKEN" "balanceOf(address)(uint256)" "$BUYER")
echo "  Provider USDC balance: $(echo "scale=2; $PROV_BAL / 10^18" | bc) USDC"
echo "  Buyer USDC balance:   $(echo "scale=2; $BUY_BAL / 10^18" | bc) USDC"
echo ""

echo -e "${GREEN}${BOLD}✓ Demo flow complete!${NC}"
echo ""
echo "  Next steps:"
echo "  - Open http://localhost:3000/jobs/$JOB_ID to view the job"
echo "  - Run the arbitration demo: bash scripts/demo-arbitration.sh"
echo "  - Run the full integration test: cd contracts && forge test"
echo ""