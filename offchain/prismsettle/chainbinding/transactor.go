package chainbinding

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// TransactorConfig holds the signing key + chain id needed to build a
// *bind.TransactOpts for write operations. The same config can be shared
// across multiple bindings if they share the same signer (e.g. Evaluator
// signs both Job.complete and Registry.submitValidation).
type TransactorConfig struct {
	PrivateKeyHex string // hex-encoded, no 0x prefix
	ChainID       *big.Int
}

// NewTransactor builds a *bind.TransactOpts from a hex private key.
//
// The caller is responsible for setting GasLimit / GasPrice / Nonce on the
// returned opts if defaults are insufficient. bind.NewKeyedTransactorWithChainID
// already sets reasonable SuggestGasPrice / SuggestGasTipCap defaults via the
// backend, but for Monad testnet you may want to override.
func NewTransactor(cfg TransactorConfig, client *ethclient.Client) (*bind.TransactOpts, error) {
	if cfg.PrivateKeyHex == "" {
		return nil, fmt.Errorf("transactor: private key is required")
	}
	if cfg.ChainID == nil || cfg.ChainID.Sign() <= 0 {
		return nil, fmt.Errorf("transactor: invalid chain id")
	}
	pk, err := crypto.HexToECDSA(cfg.PrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("transactor: parse private key: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(pk, cfg.ChainID)
	if err != nil {
		return nil, fmt.Errorf("transactor: keyed transactor: %w", err)
	}
	// Let the backend suggest gas price + nonce. The caller can override
	// these fields before invoking a Transact call if needed.
	auth.GasLimit = 0 // 0 = estimate
	return auth, nil
}

// parseABI parses a JSON ABI string. Returns a descriptive error on failure.
func parseABI(json string) (abi.ABI, error) {
	parsed, err := abi.JSON(strings.NewReader(json))
	if err != nil {
		return abi.ABI{}, fmt.Errorf("parse abi: %w", err)
	}
	return parsed, nil
}

// boundContract is a small helper that wraps bind.NewBoundContract so each
// binding file does not repeat the boilerplate.
type boundContract struct {
	addr common.Address
	abi  abi.ABI
	bind *bind.BoundContract
}

// newBoundContract builds a BoundContract for the given address + ABI on the
// given backend. The backend must be non-nil; both read and write calls go
// through it.
func newBoundContract(addr common.Address, abiJSON string, client *ethclient.Client) (boundContract, error) {
	parsed, err := parseABI(abiJSON)
	if err != nil {
		return boundContract{}, err
	}
	return boundContract{
		addr: addr,
		abi:  parsed,
		bind: bind.NewBoundContract(addr, parsed, client, client, client),
	}, nil
}
