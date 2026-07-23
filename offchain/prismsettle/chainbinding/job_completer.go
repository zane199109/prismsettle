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

// JobCompleterBinding implements evaluator.JobCompleter by calling
// PrismSettleJob.complete(jobId) on-chain. The caller (Evaluator) must hold
// COMMERCE_EVALUATOR_ROLE on the Job contract.
type JobCompleterBinding struct {
	contract boundContract
	auth     *bind.TransactOpts
}

// NewJobCompleter builds a JobCompleter bound to the PrismSettleJob contract
// at jobAddr. auth must be a *bind.TransactOpts derived from a key that has
// COMMERCE_EVALUATOR_ROLE.
func NewJobCompleter(jobAddr common.Address, client *ethclient.Client, auth *bind.TransactOpts) (*JobCompleterBinding, error) {
	bc, err := newBoundContract(jobAddr, jobABI, client)
	if err != nil {
		return nil, fmt.Errorf("job completer: %w", err)
	}
	return &JobCompleterBinding{contract: bc, auth: auth}, nil
}

// Compile-time assertion.
var _ evaluator.JobCompleter = (*JobCompleterBinding)(nil)

// Complete calls PrismSettleJob.complete(jobId). Returns the tx hash hex.
//
// Uses Transact rather than the abigen-generated wrapper so we avoid pulling
// in the full contract binding. The signer in auth already encodes the
// correct chain ID (see NewTransactor).
//
// Receipt waiting is intentionally omitted — the Evaluator records the
// decision optimistically and relies on reorg rollback to invalidate it if
// the tx is orphaned. This matches the project's "record-then-rollback"
// reorg strategy (SD §4.5.7).
func (j *JobCompleterBinding) Complete(ctx context.Context, jobID *big.Int) (string, error) {
	tx, err := j.contract.bind.Transact(j.auth, "complete", jobID)
	if err != nil {
		return "", fmt.Errorf("job complete transact: %w", err)
	}
	return tx.Hash().Hex(), nil
}
