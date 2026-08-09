package demo

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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
   "outputs":[]},
  {"name":"getJobState","type":"function","stateMutability":"view",
   "inputs":[{"name":"jobId","type":"uint256"}],
   "outputs":[
     {"name":"state","type":"uint8"},{"name":"buyer","type":"address"},
     {"name":"provider","type":"address"},{"name":"amount","type":"uint256"},
     {"name":"deliverableHash","type":"bytes32"},{"name":"proofHash","type":"bytes32"},
     {"name":"deadline","type":"uint64"},{"name":"hook","type":"address"},
     {"name":"minProviderReputation","type":"uint96"},
     {"name":"disputeResolvedAt","type":"uint256"}]},
  {"name":"getJobAmount","type":"function","stateMutability":"view",
   "inputs":[{"name":"jobId","type":"uint256"}],
   "outputs":[{"name":"","type":"uint256"}]},
  {"name":"getJobPaymentToken","type":"function","stateMutability":"view",
   "inputs":[{"name":"jobId","type":"uint256"}],
   "outputs":[{"name":"","type":"address"}]}
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

// demoTokenABI: ERC-20 approve + mint (mock token, deployer can mint) plus
// WETH9-style deposit/transfer for WMON (no mint — wrapping native MON).
const demoTokenABI = `[
  {"name":"approve","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],
   "outputs":[{"name":"","type":"bool"}]},
  {"name":"mint","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],
   "outputs":[]},
  {"name":"balanceOf","type":"function","stateMutability":"view",
   "inputs":[{"name":"owner","type":"address"}],
   "outputs":[{"name":"","type":"uint256"}]},
  {"name":"deposit","type":"function","stateMutability":"payable",
   "inputs":[],"outputs":[]},
  {"name":"transfer","type":"function","stateMutability":"nonpayable",
   "inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],
   "outputs":[{"name":"","type":"bool"}]}
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
	TokenAddr     common.Address // default payment token (USDC mock)
	WmonAddr      common.Address // optional Wrapped MON; zero when unset
	buyerAuth     *bind.TransactOpts
	providerAuth  *bind.TransactOpts
	juniorAuth    *bind.TransactOpts
	rookieAuth    *bind.TransactOpts
	evaluatorAuth *bind.TransactOpts
	deployerAuth  *bind.TransactOpts

	jobContract   *chainbinding.BoundContract
	hookContract  *chainbinding.BoundContract
	tokenContract *chainbinding.BoundContract // default token (USDC mock, mintable)
	wmonContract  *chainbinding.BoundContract // WMON (deposit/transfer); nil when unset
}

// ActionsConfig carries contract addresses and signing keys (hex, 0x optional).
type ActionsConfig struct {
	RPCURL       string
	ChainID      int64
	JobAddr      string
	HookAddr     string
	TokenAddr    string
	// WmonAddr is the Wrapped MON contract (optional). When set, the demo
	// accepts WMON as an alternative per-job token.
	WmonAddr string
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
	var wmonContract *chainbinding.BoundContract
	if cfg.WmonAddr != "" {
		wmonContract, err = chainbinding.NewBoundContract(common.HexToAddress(cfg.WmonAddr), demoTokenABI, client)
		if err != nil {
			return nil, fmt.Errorf("demo wmon binding: %w", err)
		}
	}
	return &Actions{
		client:        client,
		chainID:       chainID,
		JobAddr:       common.HexToAddress(cfg.JobAddr),
		HookAddr:      common.HexToAddress(cfg.HookAddr),
		TokenAddr:     common.HexToAddress(cfg.TokenAddr),
		WmonAddr:      common.HexToAddress(cfg.WmonAddr),
		buyerAuth:     buyerAuth,
		providerAuth:  providerAuth,
		juniorAuth:    juniorAuth,
		rookieAuth:    rookieAuth,
		evaluatorAuth: evaluatorAuth,
		deployerAuth:  deployerAuth,
		jobContract:   jobContract,
		hookContract:  hookContract,
		tokenContract: tokenContract,
		wmonContract:  wmonContract,
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
// than block base fee" reverts). Values are sanity-clamped: a misbehaving
// RPC node can return an absurd gas price, which would make the tx cost
// exceed the wallet balance ("Signer had insufficient balance").
func applyGasDefaults(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts) error {
	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		tip = big.NewInt(1_000_000_000)
	}
	feeCap, err := client.SuggestGasPrice(ctx)
	if err != nil {
		feeCap = big.NewInt(100_000_000_000)
	}
	// Clamp: Monad testnet base fee is ~100 gwei. Anything above 1,000 gwei
	// is a broken node answer, not a real fee.
	const maxFeeCap = uint64(1_000_000_000_000) // 1,000 gwei
	const maxTip = uint64(100_000_000_000)      // 100 gwei
	if feeCap.Cmp(new(big.Int).SetUint64(maxFeeCap)) > 0 {
		feeCap = new(big.Int).SetUint64(maxFeeCap)
	}
	if tip.Cmp(new(big.Int).SetUint64(maxTip)) > 0 {
		tip = new(big.Int).SetUint64(maxTip)
	}
	// fee cap must be >= base fee + tip; suggestGasPrice already includes a
	// buffer, add tip on top to be safe.
	feeCap = new(big.Int).Add(feeCap, tip)
	auth.GasTipCap = tip
	auth.GasFeeCap = feeCap
	// Fixed gasLimit instead of eth_estimateGas: some Monad RPC nodes return
	// gas=0 (or error) for transactions that are EXPECTED to revert (the grab
	// competition's losing grabs), which makes the tx fail to send with
	// "intrinsic gas greater than limit". 1M covers every demo write tx —
	// complete() alone needs ~392k — and EIP-1559 only charges the gas
	// actually used, so the headroom costs nothing extra.
	auth.GasLimit = 1_000_000
	return nil
}

// AgentIDs derive each role's agentId = uint256(wallet address) — the
// registry convention, so no configuration can drift.
func (a *Actions) BuyerAgentID() *big.Int     { return new(big.Int).SetBytes(a.buyerAuth.From.Bytes()) }
func (a *Actions) ProviderAgentID() *big.Int  { return new(big.Int).SetBytes(a.providerAuth.From.Bytes()) }
func (a *Actions) JuniorAgentID() *big.Int    { return new(big.Int).SetBytes(a.juniorAuth.From.Bytes()) }
func (a *Actions) RookieAgentID() *big.Int    { return new(big.Int).SetBytes(a.rookieAuth.From.Bytes()) }
func (a *Actions) EvaluatorAgentID() *big.Int { return new(big.Int).SetBytes(a.evaluatorAuth.From.Bytes()) }

// BuyerAddress returns the demo buyer wallet (BUYER_KEY) — used to verify
// that an existing job was created by the demo wallet before resuming it.
func (a *Actions) BuyerAddress() common.Address { return a.buyerAuth.From }

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
	// Refresh EIP-1559 fee fields before EVERY send: Monad's base fee moves
	// and the multi-node RPC pool can answer differently per call. Stale
	// values either underpay ("max fee per gas less than block base fee") or,
	// if a broken node answered at startup, overprice the tx so hard the
	// wallet looks broke.
	if err := applyGasDefaults(ctx, a.client, auth); err != nil {
		return "", fmt.Errorf("gas defaults: %w", err)
	}
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
		// Replay with eth_call to surface the real revert reason (receipts
		// carry no revert data). Without this, every failed tx shows as a
		// bare "tx reverted: <hash>" and demo feedback loses its meaning.
		if reason := a.revertReasonFor(ctx, contract, auth, method, args...); reason != "" {
			return tx.Hash().Hex(), fmt.Errorf("tx reverted: %s", reason)
		}
		return tx.Hash().Hex(), fmt.Errorf("tx reverted: %s", tx.Hash().Hex())
	}
	return tx.Hash().Hex(), nil
}

// revertReasonFor replays a call with eth_call to extract the revert reason
// from a failed transaction. Empty string when the call unexpectedly succeeds
// or the node returns nothing parseable.
func (a *Actions) revertReasonFor(ctx context.Context, contract *chainbinding.BoundContract, auth *bind.TransactOpts, method string, args ...interface{}) string {
	var out []interface{}
	err := contract.Raw().Call(&bind.CallOpts{
		Context: ctx, From: auth.From,
	}, &out, method, args...)
	if err == nil {
		return ""
	}
	msg := err.Error()
	if idx := strings.Index(msg, "reverted: "); idx >= 0 {
		return strings.TrimSpace(msg[idx+len("reverted: "):])
	}
	return msg
}

// Token helpers — resolve the contract + symbol for a per-job token.
func (a *Actions) isWmon(addr common.Address) bool {
	return a.WmonAddr != (common.Address{}) && addr == a.WmonAddr
}

// tokenSymbol maps a per-job token address to a display symbol.
func (a *Actions) tokenSymbol(addr common.Address) string {
	if a.isWmon(addr) {
		return "WMON"
	}
	return "USDC"
}

// ValidateToken returns an error when the address is not a supported
// per-job token (default USDC mock or WMON).
func (a *Actions) ValidateToken(addr common.Address) error {
	_, err := a.tokenBinding(addr)
	return err
}

// tokenBinding returns the bound contract for a per-job token address.
// Only the default token (USDC mock) and WMON (when configured) are allowed.
func (a *Actions) tokenBinding(addr common.Address) (*chainbinding.BoundContract, error) {
	if a.isWmon(addr) {
		if a.wmonContract == nil {
			return nil, fmt.Errorf("wmon not configured")
		}
		return a.wmonContract, nil
	}
	if addr == a.TokenAddr {
		return a.tokenContract, nil
	}
	return nil, fmt.Errorf("unsupported token %s", addr.Hex())
}

// transactWithValue sends a write tx with msg.value (payable calls such as
// WMON.deposit) and waits for the receipt. The caller's auth is cloned so
// its Value field is never mutated (auths are reused across steps).
func (a *Actions) transactWithValue(ctx context.Context, contract *chainbinding.BoundContract, auth *bind.TransactOpts, value *big.Int, method string, args ...interface{}) (string, error) {
	bc := contract.Raw()
	clone := *auth
	clone.Value = value
	tx, err := bc.Transact(&clone, method, args...)
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

// EnsureNativeGas tops up native MON (gas) for every demo role wallet when
// its balance drops below the threshold. Demo roles burn gas on every script
// step; after many runs their MON runs out and any write tx fails with
// "insufficient balance" (signer-level rejection, before the contract even
// sees it). The deployer funds them — it holds plenty from deployment.
func (a *Actions) EnsureNativeGas(ctx context.Context) error {
	const minGas = uint64(500_000_000_000_000_000) // 0.5 MON
	const topUp = uint64(1_000_000_000_000_000_000) // 1 MON
	for _, addr := range []common.Address{
		a.buyerAuth.From, a.providerAuth.From, a.juniorAuth.From,
		a.rookieAuth.From, a.evaluatorAuth.From,
	} {
		bal, err := a.client.BalanceAt(ctx, addr, nil)
		if err != nil {
			return fmt.Errorf("native balance check %s: %w", addr.Hex(), err)
		}
		if bal.Cmp(new(big.Int).SetUint64(minGas)) >= 0 {
			continue
		}
		if _, err := a.sendNative(ctx, addr, new(big.Int).SetUint64(topUp)); err != nil {
			return fmt.Errorf("native top-up %s: %w", addr.Hex(), err)
		}
	}
	return nil
}

// sendNative transfers native MON from the deployer wallet (plain 21000-gas
// value transfer, signed via the deployer's TransactOpts signer).
func (a *Actions) sendNative(ctx context.Context, to common.Address, amount *big.Int) (string, error) {
	nonce, err := a.client.PendingNonceAt(ctx, a.deployerAuth.From)
	if err != nil {
		return "", err
	}
	if a.deployerAuth.GasFeeCap == nil || a.deployerAuth.GasTipCap == nil {
		if err := applyGasDefaults(ctx, a.client, a.deployerAuth); err != nil {
			return "", err
		}
	}
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   a.chainID,
		Nonce:     nonce,
		To:        &to,
		Value:     amount,
		Gas:       21000,
		GasFeeCap: a.deployerAuth.GasFeeCap,
		GasTipCap: a.deployerAuth.GasTipCap,
	})
	signed, err := a.deployerAuth.Signer(a.deployerAuth.From, tx)
	if err != nil {
		return "", err
	}
	if err := a.client.SendTransaction(ctx, signed); err != nil {
		return "", err
	}
	receipt, err := bind.WaitMined(ctx, a.client, signed)
	if err != nil {
		return "", err
	}
	if receipt.Status != 1 {
		return signed.Hash().Hex(), fmt.Errorf("native transfer reverted")
	}
	return signed.Hash().Hex(), nil
}

// EnsureFunds tops up the buyer (escrow + reject deposit + dispute deposit)
// and the provider (dispute deposit) in the given per-job token. USDC is
// minted by the deployer; WMON is wrapped from native MON and transferred.
// Balances are checked first (safe to re-run across sessions).
func (a *Actions) EnsureFunds(ctx context.Context, amount *big.Int, token common.Address) error {
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
		bal, err := a.tokenBalance(ctx, token, rec.addr)
		if err != nil {
			return fmt.Errorf("balance check: %w", err)
		}
		if bal.Cmp(rec.amount) >= 0 {
			continue
		}
		topUp := new(big.Int).Sub(rec.amount, bal)
		if err := a.topUp(ctx, token, rec.addr, topUp); err != nil {
			return err
		}
	}
	return nil
}

// topUp funds a wallet in the given token: mint for the USDC mock, wrap
// native MON + transfer for WMON (WMON has no mint; deposit is payable).
func (a *Actions) topUp(ctx context.Context, token common.Address, to common.Address, amount *big.Int) error {
	if a.isWmon(token) {
		if _, err := a.transactWithValue(ctx, a.wmonContract, a.deployerAuth, amount, "deposit"); err != nil {
			return fmt.Errorf("wmon wrap: %w", err)
		}
		if _, err := a.transact(ctx, a.wmonContract, a.deployerAuth, "transfer", to, amount); err != nil {
			return fmt.Errorf("wmon transfer: %w", err)
		}
		return nil
	}
	if _, err := a.transact(ctx, a.tokenContract, a.deployerAuth, "mint", to, amount); err != nil {
		return fmt.Errorf("mint %s: %w", to.Hex(), err)
	}
	return nil
}

func (a *Actions) tokenBalance(ctx context.Context, token, addr common.Address) (*big.Int, error) {
	bound, err := a.tokenBinding(token)
	if err != nil {
		return nil, err
	}
	bc := bound.Raw()
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
// token selects the per-job payment currency: the default token (USDC mock,
// createJob passes 0x0 = contract default) or WMON (passed explicitly).
// Returns the jobId (from the JobCreated log) and the tx hashes.
func (a *Actions) CreateAndFund(
	ctx context.Context,
	agentID *big.Int,
	deadline uint64,
	minRep *big.Int,
	amount *big.Int,
	token common.Address,
) (jobID *big.Int, txHashes []string, err error) {
	tok, err := a.tokenBinding(token)
	if err != nil {
		return nil, nil, err
	}
	paymentToken := common.Address{}
	if a.isWmon(token) {
		paymentToken = a.WmonAddr
	}
	bc := a.jobContract.Raw()
	tx, err := bc.Transact(a.buyerAuth, "createJob", agentID, big.NewInt(0), deadline, a.HookAddr, minRep, paymentToken)
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
	if _, err := a.transact(ctx, tok, a.buyerAuth, "approve", a.JobAddr, amount); err != nil {
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
		if _, err := a.transact(ctx, tok, auth, "approve", a.HookAddr, maxApproval); err != nil {
			return nil, nil, fmt.Errorf("approve hook deposit: %w", err)
		}
	}
	return jobID, txHashes, nil
}

// Grab claims the job as the provider agent (owner check enforced on-chain).
func (a *Actions) Grab(ctx context.Context, jobID, providerAgentID *big.Int) (string, error) {
	return a.transact(ctx, a.jobContract, a.providerAuth, "grabJob", jobID, providerAgentID)
}

// JobState mirrors the on-chain JobState enum (0=Created … 6=Refunded) plus
// the fields the demo needs to resume an existing job.
type JobState struct {
	State         uint8
	Buyer         common.Address
	Provider      common.Address
	Amount        *big.Int
	MinProviderRep *big.Int // uint96, 1e18-scaled
}

// ReadJobState fetches the full job state via getJobState (eth_call).
func (a *Actions) ReadJobState(ctx context.Context, jobID *big.Int) (*JobState, error) {
	bc := a.jobContract.Raw()
	var outs []interface{}
	if err := bc.Call(&bind.CallOpts{Context: ctx}, &outs, "getJobState", jobID); err != nil {
		return nil, err
	}
	if len(outs) < 10 {
		return nil, fmt.Errorf("getJobState: bad output length %d", len(outs))
	}
	st := &JobState{}
	if v, ok := outs[0].(uint8); ok {
		st.State = v
	}
	if v, ok := outs[1].(common.Address); ok {
		st.Buyer = v
	}
	if v, ok := outs[2].(common.Address); ok {
		st.Provider = v
	}
	if v, ok := outs[3].(*big.Int); ok {
		st.Amount = v
	}
	if v, ok := outs[8].(*big.Int); ok {
		st.MinProviderRep = v
	}
	if st.Amount == nil {
		st.Amount = big.NewInt(0)
	}
	if st.MinProviderRep == nil {
		st.MinProviderRep = big.NewInt(0)
	}
	return st, nil
}

// ReadJobToken returns the job's effective payment token address.
func (a *Actions) ReadJobToken(ctx context.Context, jobID *big.Int) (common.Address, error) {
	bc := a.jobContract.Raw()
	var outs []interface{}
	if err := bc.Call(&bind.CallOpts{Context: ctx}, &outs, "getJobPaymentToken", jobID); err != nil {
		return common.Address{}, err
	}
	if len(outs) == 0 {
		return common.Address{}, fmt.Errorf("getJobPaymentToken: no output")
	}
	addr, ok := outs[0].(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf("getJobPaymentToken: bad output type")
	}
	return addr, nil
}

// PreflightGrab simulates grabJob from the given signer (eth_call, no gas)
// and returns the revert reason — the orchestrator uses it to pick the grab
// competition script based on why a competitor would lose.
func (a *Actions) PreflightGrab(ctx context.Context, jobID, agentID *big.Int, from common.Address) error {
	bc := a.jobContract.Raw()
	var outs []interface{}
	return bc.Call(&bind.CallOpts{Context: ctx, From: from}, &outs, "grabJob", jobID, agentID)
}

// FundExisting funds an already-created job (buyer signs): approve the job
// contract for the escrow amount, then fundViaToken.
func (a *Actions) FundExisting(ctx context.Context, jobID *big.Int, amount *big.Int, token common.Address) (string, error) {
	tok, err := a.tokenBinding(token)
	if err != nil {
		return "", err
	}
	if _, err := a.transact(ctx, tok, a.buyerAuth, "approve", a.JobAddr, amount); err != nil {
		return "", fmt.Errorf("approve job: %w", err)
	}
	return a.transact(ctx, a.jobContract, a.buyerAuth, "fundViaToken", jobID, amount, []byte{})
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
