package main

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Minimal hand-written binding for PrismSettleRegistry.registerAgent. Avoids
// pulling in abigen output for a single-function call. If more functions are
// needed later, generate a proper binding via `abigen --abi registry.json`.
//
// ABI signature: registerAgent(uint256 agentId, string metadata)
const registryABI = `[
  {"name":"registerAgent","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"agentId","type":"uint256"},{"name":"metadata","type":"string"}],
   "outputs":[]}
]`

// RegistryCaller is a minimal binding exposing only RegisterAgent.
type RegistryCaller struct {
	addr    common.Address
	backend bind.ContractBackend
	parsed  abi.ABI
}

// NewRegistryCaller builds a binding for the Registry at addr.
func NewRegistryCaller(addr common.Address, client *ethclient.Client) (*RegistryCaller, error) {
	parsed, err := abi.JSON(strings.NewReader(registryABI))
	if err != nil {
		return nil, fmt.Errorf("parse registry abi: %w", err)
	}
	return &RegistryCaller{addr: addr, backend: client, parsed: parsed}, nil
}

// RegisterAgent sends a registerAgent(agentId, metadata) transaction signed
// by auth. Returns the submitted transaction.
//
// We use bind.BoundContract.Transact so the Signer (which already encodes
// ChainID via NewKeyedTransactorWithChainID) handles 1559/legacy detection.
func (r *RegistryCaller) RegisterAgent(auth *bind.TransactOpts, agentID *big.Int, metadata string) (*types.Transaction, error) {
	contract := bind.NewBoundContract(r.addr, r.parsed, r.backend, r.backend, r.backend)
	var (
		tx  *types.Transaction
		err error
	)
	tx, err = contract.Transact(auth, "registerAgent", agentID, metadata)
	if err != nil {
		return nil, fmt.Errorf("transact registerAgent: %w", err)
	}
	return tx, nil
}
