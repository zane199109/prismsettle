package demo

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/zane/web3-offchain/prismsettle/chainbinding"
)

// demoJobABI: PrismSettleJob functions the orchestrator needs.
const demoJobABI = `[
  {"name":"createJob","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"agentId","type":"uint256"},{"name":"parentJobId","type":"uint256"},{"name":"deadline","type":"uint64"},{"name":"hook","type":"address"},{"name":"minProviderReputation","type":"uint96"},{"name":"paymentToken","type":"address"}],
   "outputs":[{"name":"jobId","type":"uint256"}]},
  {"name":"fundViaToken","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"amount","type":"uint256"},{"name":"receipt","type":"bytes"}],
   "outputs":[]},
  {"name":"grabJob","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"providerAgentId","type":"uint256"}],
   "outputs":[]},
  {"name":"submit","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"deliverableHash","type":"bytes32"},{"name":"proofHash","type":"bytes32"}],
   "outputs":[]},
  {"name":"reject","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"reasonHash","type":"bytes32"}],
   "outputs":[]},
  {"name":"complete","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"score","type":"uint96"}],
   "outputs":[]},
  {"name":"executeArbitrationResult","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"}],
   "outputs":[]}
]`

// demoHookABI: ArbitrationHook functions.
const demoHookABI = `[
  {"name":"dispute","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"reasonHash","type":"bytes32"}],
   "outputs":[]},
  {"name":"resolveDispute","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"jobId","type":"uint256"},{"name":"ruling","type":"uint8"}],
   "outputs":[]}
]`

// demoTokenABI: ERC-20 approve + mint (mock token, deployer can mint).
const demoTokenABI = `[
  {"name":"approve","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],
   "outputs":[{"name":"","type":"bool"}]},
  {"name":"mint","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],
   "outputs":[]},
  {"name":"balanceOf","type":"function","stateMutability":"view",
   "inputs":[{"name":"owner","type":"address"}],
   "outputs":[{"name":"","type":"uint256"}]}
]`

// jobCreatedTopic = keccak("JobCreated(uint256,uint256,address,uint64,address,uint96,address)")
var jobCreatedTopic = crypto.Keccak256Hash([]byte(
	"JobCreated(uint256,uint256,address,uint64,address,uint96,address)",
))

// Actions executes the on-chain steps of a demo session with real signatures.
type Actions struct {
	client        *ethclient.Client
	chainID       *big.Int
	JobAddr       common.Address
	HookAddr      common.Address
	TokenAddr     common.Address
	buyerAuth     *bind.TransactOpts
	providerAuth  *bind.TransactOpts
	juniorAuth    *bind.TransactOpts
	rookieAuth    *bind.TransactOpts
	evaluatorAuth *bind.TransactOpts
	deployerAuth  *bind.TransactOpts

	jobContract   *chainbinding.BoundContract
	hookContract  *chainbinding.BoundContract
	tokenContract *chainbinding.BoundContract
}

// ActionsConfig carries contract addresses and signing keys (hex, 0x optional).
type ActionsConfig struct {
	RPCURL       string
	ChainID      int64
	JobAddr      string
	HookAddr     string
	TokenAddr    string
	BuyerKey     string // BUYER_KEY
	ProviderKey  string // AUDITOR_SENIOR_KEY (wins the grab in the demo)
	JuniorKey    string // AUDITOR_JUNIOR_KEY (loses the grab: ownership mismatch)
	RookieKey    string // AUDITOR_ROOKIE_KEY (loses the grab: ownership mismatch)
	EvaluatorKey string // PRISM_EVALUATOR_KEY
	DeployerKey  string // DEPLOYER_KEY
}

func NewActions(cfg ActionsConfig) (*Actions, error) {
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("demo actions dial: %w", err)
	}
	chainID := new(big.Int).SetInt64(cfg.ChainID)
	buyerAuth, err := newAuth(cfg.BuyerKey, chainID)
	if err != nil {
		return nil, fmt.Errorf("demo buyer auth: %w", err)
	}
	providerAuth, err := newAuth(cfg.ProviderKey, chainID)
	if err != nil {
		return nil, fmt.Errorf("demo provider auth: %w", err)
	}
	juniorAuth, err := newAuth(cfg.JuniorKey, chainID)
	if err != nil {
		return nil, fmt.Errorf("demo junior auth: %w", err)
	}
	rookieAuth, err := newAuth(cfg.RookieKey, chainID)
	if err != nil {
		return nil, fmt.Errorf("demo rookie auth: %w", err)
	}
	evaluatorAuth, err := newAuth(cfg.EvaluatorKey, chainID)
	if err != nil {
		return nil, fmt.Errorf("demo evaluator auth: %w", err)
	}
	deployerAuth, err := newAuth(cfg.DeployerKey, chainID)
	if err != nil {
		return nil, fmt.Errorf("demo deployer auth: %w", err)
	}
	ctx := context.Background()
	for _, a := range []*bind.TransactOpts{buyerAuth, providerAuth, juniorAuth, rookieAuth, evaluatorAuth, deployerAuth} {
		if err := applyGasDefaults(ctx, client, a); err != nil {
			return nil, fmt.Errorf("demo gas defaults: %w", err)
		}
	}
	jobContract, err := chainbinding.NewBoundContract(common.HexToAddress(cfg.JobAddr), demoJobABI, client)
	if err != nil {
		return nil, fmt.Errorf("demo job binding: %w", err)
	}
	hookContract, err := chainbinding.NewBoundContract(common.HexToAddress(cfg.HookAddr), demoHookABI, client)
	if err != nil {
		return nil, fmt.Errorf("demo hook binding: %w", err)
	}
	tokenContract, err := chainbinding.NewBoundContract(common.HexToAddress(cfg.TokenAddr), demoTokenABI, client)
	if err != nil {
		return nil, fmt.Errorf("demo token binding: %w", err)
	}
	return &Actions{
		client:        client,
		chainID:       chainID,
		JobAddr:       common.HexToAddress(cfg.JobAddr),
		HookAddr:      common.HexToAddress(cfg.HookAddr),
		TokenAddr:     common.HexToAddress(cfg.TokenAddr),
		buyerAuth:     buyerAuth,
		providerAuth:  providerAuth,
		juniorAuth:    juniorAuth,
		rookieAuth:    rookieAuth,
		evaluatorAuth: evaluatorAuth,
		deployerAuth:  deployerAuth,
		jobContract:   jobContract,
		hookContract:  hookContract,
		tokenContract: tokenContract,
	}, nil
}

func newAuth(keyHex string, chainID *big.Int) (*bind.TransactOpts, error) {
	if keyHex == "" {
		return nil, fmt.Errorf("missing private key")
	}
	pk, err := crypto.HexToECDSA(strings.TrimPrefix(keyHex, "0x"))
	if err != nil {
		return nil, err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(pk, chainID)
	if err != nil {
		return nil, err
	}
	// EIP-1559: clear the default GasPrice (set by NewKeyedTransactor);
	// tip + fee cap are set dynamically in NewActions from the chain.
	auth.GasPrice = nil
	return auth, nil
}

// applyGasDefaults sets EIP-1559 fee fields from chain suggestions (Monad
// testnet base fee fluctuates; a fixed cap causes "max fee per gas less
// than block base fee" reverts).
func applyGasDefaults(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts) error {
	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		tip = big.NewInt(1_000_000_000)
	}
	feeCap, err := client.SuggestGasPrice(ctx)
	if err != nil {
		feeCap = big.NewInt(100_000_000_000)
	}
	// fee cap must be >= base fee + tip; suggestGasPrice already includes a
	// buffer, add tip on top to be safe.
	feeCap = new(big.Int).Add(feeCap, tip)
	auth.GasTipCap = tip
	auth.GasFeeCap = feeCap
	return nil
}

// AgentIDs derive each role's agentId = uint256(wallet address) — the
// registry convention, so no configuration can drift.
func (a *Actions) BuyerAgentID() *big.Int     { return new(big.Int).SetBytes(a.buyerAuth.From.Bytes()) }
func (a *Actions) ProviderAgentID() *big.Int  { return new(big.Int).SetBytes(a.providerAuth.From.Bytes()) }
func (a *Actions) JuniorAgentID() *big.Int    { return new(big.Int).SetBytes(a.juniorAuth.From.Bytes()) }
func (a *Actions) RookieAgentID() *big.Int    { return new(big.Int).SetBytes(a.rookieAuth.From.Bytes()) }
func (a *Actions) EvaluatorAgentID() *big.Int { return new(big.Int).SetBytes(a.evaluatorAuth.From.Bytes()) }

// JuniorAuth / RookieAuth expose the competitor signers for the grab race.
func (a *Actions) JuniorAuth() *bind.TransactOpts { return a.juniorAuth }
func (a *Actions) RookieAuth() *bind.TransactOpts { return a.rookieAuth }

// GrabCompeting tries to claim the job with a mismatched agent ID so the
// registry ownership check reverts — a real, on-chain "lost the race" path
// used by the demo's grab competition (junior/rookie lose, senior wins).
// The returned error is the revert reason surfaced in the chat.
func (a *Actions) GrabCompeting(ctx context.Context, jobID, wrongAgentID *big.Int, auth *bind.TransactOpts) (string, error) {
	return a.transact(ctx, a.jobContract, auth, "grabJob", jobID, wrongAgentID)
}

// transact sends a write tx and waits for the receipt; returns tx hash.
func (a *Actions) transact(ctx context.Context, contract *chainbinding.BoundContract, auth *bind.TransactOpts, method string, args ...interface{}) (string, error) {
	bc := contract.Raw()
	tx, err := bc.Transact(auth, method, args...)
	if err != nil {
		return "", err
	}
	receipt, err := bind.WaitMined(ctx, a.client, tx)
	if err != nil {
		return "", err
	}
	if receipt.Status != 1 {
		return tx.Hash().Hex(), fmt.Errorf("tx reverted: %s", tx.Hash().Hex())
	}
	return tx.Hash().Hex(), nil
}

// EnsureFunds tops up the buyer (escrow + reject deposit + dispute deposit)
// and the provider (dispute deposit) with mock tokens via the deployer.
// Safe on testnet where the token is a deployer-mintable mock; balances are
// checked first.
func (a *Actions) EnsureFunds(ctx context.Context, amount *big.Int) error {
	deposit := new(big.Int).Div(new(big.Int).Mul(amount, big.NewInt(500)), big.NewInt(10000))
	// Buyer spends: escrow + first-reject deposit + dispute deposit.
	buyerNeed := new(big.Int).Add(amount, new(big.Int).Mul(deposit, big.NewInt(2)))
	for _, rec := range []struct {
		addr   common.Address
		amount *big.Int
	}{
		{a.buyerAuth.From, buyerNeed},
		{a.providerAuth.From, deposit},
		{a.evaluatorAuth.From, deposit},
	} {
		bal, err := a.tokenBalance(ctx, rec.addr)
		if err != nil {
			return fmt.Errorf("balance check: %w", err)
		}
		if bal.Cmp(rec.amount) >= 0 {
			continue
		}
		topUp := new(big.Int).Sub(rec.amount, bal)
		if _, err := a.transact(ctx, a.tokenContract, a.deployerAuth, "mint", rec.addr, topUp); err != nil {
			return fmt.Errorf("mint %s: %w", rec.addr.Hex(), err)
		}
	}
	return nil
}

func (a *Actions) tokenBalance(ctx context.Context, addr common.Address) (*big.Int, error) {
	bc := a.tokenContract.Raw()
	var outs []interface{}
	if err := bc.Call(&bind.CallOpts{Context: ctx}, &outs, "balanceOf", addr); err != nil {
		return nil, err
	}
	if len(outs) == 0 {
		return nil, fmt.Errorf("balanceOf: no output")
	}
	b, ok := outs[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("balanceOf: bad output type")
	}
	return b, nil
}

// CreateAndFund runs createJob + approve + fundViaToken as the buyer.
// Returns the jobId (from the JobCreated log) and the tx hashes.
func (a *Actions) CreateAndFund(
	ctx context.Context,
	agentID *big.Int,
	deadline uint64,
	minRep *big.Int,
	amount *big.Int,
) (jobID *big.Int, txHashes []string, err error) {
	bc := a.jobContract.Raw()
	tx, err := bc.Transact(a.buyerAuth, "createJob", agentID, big.NewInt(0), deadline, a.HookAddr, minRep, a.TokenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("createJob: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, a.client, tx)
	if err != nil {
		return nil, nil, fmt.Errorf("createJob mined: %w", err)
	}
	if receipt.Status != 1 {
		return nil, nil, fmt.Errorf("createJob reverted: %s", tx.Hash().Hex())
	}
	txHashes = append(txHashes, tx.Hash().Hex())
	// Extract jobId from the JobCreated log (topics: [sig, agentId, jobId]).
	for _, lg := range receipt.Logs {
		if len(lg.Topics) == 3 && lg.Topics[0] == jobCreatedTopic {
			jobID = new(big.Int).SetBytes(lg.Topics[2].Bytes())
			break
		}
	}
	if jobID == nil {
		return nil, nil, fmt.Errorf("createJob: JobCreated log not found in %s", tx.Hash().Hex())
	}

	// approve token to the job contract, then fund.
	if _, err := a.transact(ctx, a.tokenContract, a.buyerAuth, "approve", a.JobAddr, amount); err != nil {
		return nil, nil, fmt.Errorf("approve: %w", err)
	}
	txHashes = append(txHashes, "")
	fh, err := a.transact(ctx, a.jobContract, a.buyerAuth, "fundViaToken", jobID, amount, []byte{})
	if err != nil {
		return nil, nil, fmt.Errorf("fundViaToken: %w", err)
	}
	txHashes[len(txHashes)-1] = fh

	// approve the hook with a max allowance: the buyer needs allowance for
	// the first-reject deposit AND the dispute deposit; provider + evaluator
	// need allowance for dispute deposits. Max is fine for the mock token.
	maxApproval := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	for _, auth := range []*bind.TransactOpts{a.buyerAuth, a.providerAuth, a.evaluatorAuth} {
		if _, err := a.transact(ctx, a.tokenContract, auth, "approve", a.HookAddr, maxApproval); err != nil {
			return nil, nil, fmt.Errorf("approve hook deposit: %w", err)
		}
	}
	return jobID, txHashes, nil
}

// Grab claims the job as the provider agent (owner check enforced on-chain).
func (a *Actions) Grab(ctx context.Context, jobID, providerAgentID *big.Int) (string, error) {
	return a.transact(ctx, a.jobContract, a.providerAuth, "grabJob", jobID, providerAgentID)
}

// Submit delivers work (deliverable + proof hashes).
func (a *Actions) Submit(ctx context.Context, jobID *big.Int, deliverableHash, proofHash [32]byte) (string, error) {
	return a.transact(ctx, a.jobContract, a.providerAuth, "submit", jobID, deliverableHash, proofHash)
}

// Reject returns the deliverable with a reason (buyer signs; only the first
// reject posts a deposit per the contract).
func (a *Actions) Reject(ctx context.Context, jobID *big.Int, reasonHash [32]byte) (string, error) {
	return a.transact(ctx, a.jobContract, a.buyerAuth, "reject", jobID, reasonHash)
}

// Complete accepts the deliverable and settles directly (no arbitration) —
// the "direct completion" demo scenario.
func (a *Actions) Complete(ctx context.Context, jobID *big.Int, score *big.Int) (string, error) {
	return a.transact(ctx, a.jobContract, a.buyerAuth, "complete", jobID, score)
}

// Dispute opens arbitration (provider signs — deliverable deemed compliant).
func (a *Actions) Dispute(ctx context.Context, jobID *big.Int, reasonHash [32]byte) (string, error) {
	return a.transact(ctx, a.hookContract, a.providerAuth, "dispute", jobID, reasonHash)
}

// Resolve rules on the dispute (evaluator signs; ruling 2 = provider wins).
func (a *Actions) Resolve(ctx context.Context, jobID *big.Int, ruling uint8) (string, error) {
	return a.transact(ctx, a.hookContract, a.evaluatorAuth, "resolveDispute", jobID, ruling)
}

// Execute settles the escrow after the announcement period (anyone; deployer).
func (a *Actions) Execute(ctx context.Context, jobID *big.Int) (string, error) {
	return a.transact(ctx, a.jobContract, a.deployerAuth, "executeArbitrationResult", jobID)
}

// Close releases the RPC connection.
func (a *Actions) Close() {
	if a.client != nil {
		a.client.Close()
	}
}

// WaitForAnnouncement sleeps through ANNOUNCEMENT_PERIOD (60s on testnet).
func WaitForAnnouncement(ctx context.Context) error {
	select {
	case <-time.After(65 * time.Second):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// hash32 is a helper to build a bytes32 from a hex string.
func hash32(hex string) [32]byte {
	var out [32]byte
	copy(out[:], common.FromHex(hex))
	return out
}
