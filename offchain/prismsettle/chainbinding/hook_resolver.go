package chainbinding

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zane/web3-offchain/prismsettle/evaluator"
)

// HookResolverBinding implements evaluator.HookResolver by calling
// ArbitrationHook.resolveDispute(jobId, ruling) on-chain. The caller
// (Arbitrator) must hold RESOLVER_ROLE on the Hook contract.
type HookResolverBinding struct {
	contract boundContract
	auth     *bind.TransactOpts
}

// NewHookResolver builds a HookResolver bound to the ArbitrationHook contract
// at hookAddr. auth must be a *bind.TransactOpts derived from a key that has
// RESOLVER_ROLE.
func NewHookResolver(hookAddr common.Address, client *ethclient.Client, auth *bind.TransactOpts) (*HookResolverBinding, error) {
	bc, err := newBoundContract(hookAddr, hookABI, client)
	if err != nil {
		return nil, fmt.Errorf("hook resolver: %w", err)
	}
	return &HookResolverBinding{contract: bc, auth: auth}, nil
}

// Compile-time assertion.
var _ evaluator.HookResolver = (*HookResolverBinding)(nil)

// ResolveDispute calls ArbitrationHook.resolveDispute(jobId, ruling).
// Returns the tx hash hex.
//
// ruling values (SD §4.5.4):
//   - 0 = Refund buyer (provider fault)
//   - 1 = Pay provider (buyer dispute rejected)
//   - 2 = Split (partial refund + partial payment)
func (h *HookResolverBinding) ResolveDispute(ctx context.Context, jobID *big.Int, ruling uint8) (string, error) {
	tx, err := h.contract.bind.Transact(h.auth, "resolveDispute", jobID, ruling)
	if err != nil {
		return "", fmt.Errorf("resolve dispute transact: %w", err)
	}
	return tx.Hash().Hex(), nil
}
