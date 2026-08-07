package chainbinding

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zane/web3-offchain/prismsettle/evaluator"
)

// RegistryWriterBinding implements evaluator.RegistryWriter by calling
// PrismSettleRegistry.submitValidation(agentId, score, proofHash, jobId,
// source) on-chain. The caller (Evaluator/Arbitrator) must hold
// REGISTRY_EVALUATOR_ROLE on the Registry contract.
type RegistryWriterBinding struct {
	contract boundContract
	auth     *bind.TransactOpts
}

// NewRegistryWriter builds a RegistryWriter bound to the PrismSettleRegistry
// contract at regAddr. auth must be a *bind.TransactOpts derived from a key
// that has REGISTRY_EVALUATOR_ROLE.
func NewRegistryWriter(regAddr common.Address, client *ethclient.Client, auth *bind.TransactOpts) (*RegistryWriterBinding, error) {
	bc, err := newBoundContract(regAddr, registryABI, client)
	if err != nil {
		return nil, fmt.Errorf("registry writer: %w", err)
	}
	return &RegistryWriterBinding{contract: bc, auth: auth}, nil
}

// Compile-time assertion.
var _ evaluator.RegistryWriter = (*RegistryWriterBinding)(nil)

// SubmitValidation calls Registry.submitValidation(agentId, score, proofHash,
// jobId, source). Returns the tx hash hex.
//
// proofHash is a bytes32 hex string (0x-prefixed, 64 hex chars). It is
// converted to common.Hash before passing to the ABI encoder.
func (r *RegistryWriterBinding) SubmitValidation(
	ctx context.Context,
	agentID *big.Int,
	score uint64,
	proofHash string,
	jobID *big.Int,
	source uint8,
) (string, error) {
	hash, err := parseBytes32(proofHash)
	if err != nil {
		return "", fmt.Errorf("registry writer: parse proof hash: %w", err)
	}
	tx, err := r.contract.bind.Transact(r.auth, "submitValidation", agentID, score, hash, jobID, source)
	if err != nil {
		return "", fmt.Errorf("submit validation transact: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// SetAggregatedScore calls Registry.setAggregatedScore(agentId, newScore).
// Returns the tx hash hex. Used by the arbitration path to apply penalty
// scores directly to a provider's aggregated reputation (FR-E13).
func (r *RegistryWriterBinding) SetAggregatedScore(ctx context.Context, agentID *big.Int, newScore uint64) (string, error) {
	tx, err := r.contract.bind.Transact(r.auth, "setAggregatedScore", agentID, newScore)
	if err != nil {
		return "", fmt.Errorf("set aggregated score transact: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// parseBytes32 converts a 0x-prefixed 64-char hex string to common.Hash.
// Accepts both 0x-prefixed and unprefixed forms; pads/truncates to 32 bytes.
func parseBytes32(s string) (common.Hash, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return common.Hash{}, nil
	}
	if !strings.HasPrefix(s, "0x") {
		s = "0x" + s
	}
	if len(s) != 66 { // 0x + 64 chars
		return common.Hash{}, fmt.Errorf("bytes32 must be 32 bytes (66 chars with 0x), got %d", len(s))
	}
	return common.HexToHash(s), nil
}
