// Package main implements a smart-contract auditor worker agent for the
// PrismSettle demo. It polls the offchain REST API for available jobs,
// competes by reputation to grab them, and submits mock audit deliverables.
package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zane/web3-offchain/pkg/config"
	"github.com/zane/web3-offchain/pkg/logger"
	"github.com/zane/web3-offchain/prismsettle/chainbinding"
)

// JobState enum matching PrismSettleJob.sol
const (
	JobStateCreated   = 0
	JobStateFunded    = 1
	JobStateAssigned  = 2
	JobStateSubmitted = 3
)

// envOrDefault reads an env var or returns a default value.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// PrismEventItem represents a single event from the offchain API.
// JSON tags must match the backend ChainEvent serialization
// (event_type/job_id/tx_hash/...). ID is the DB cursor for incremental pulls.
type PrismEventItem struct {
	ID        uint64 `json:"id"`
	EventType string `json:"event_type"`
	JobID     string `json:"job_id"`
	From      string `json:"from"`
	TxHash    string `json:"tx_hash"`
	LogIndex  uint   `json:"log_index"`
	RawData   string `json:"raw_data"`
	ChainName string `json:"chain_name"`
	CreatedAt string `json:"created_at"`
}

// PrismEventsResponse wraps the paginated event list. Backend shape:
// {"code":0,"data":{"list":[...],"total":N,"page":1,"size":20}}.
type PrismEventsResponse struct {
	Data struct {
		List []PrismEventItem `json:"list"`
	} `json:"data"`
}

func main() {
	agentIDStr := os.Getenv("AGENT_ID")
	// AGENT_ID is deprecated: the agent id derives from the wallet
	// (agentId = uint256(owner address)) so configuration can never drift
	// from the signing key. If set, it is only used for a sanity check.
	agentPrivKey := os.Getenv("AGENT_PRIVATE_KEY")
	if agentPrivKey == "" {
		fmt.Fprintln(os.Stderr, "error: AGENT_PRIVATE_KEY is required")
		os.Exit(1)
	}
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		fmt.Fprintln(os.Stderr, "error: RPC_URL is required")
		os.Exit(1)
	}
	chainIDStr := os.Getenv("CHAIN_ID")
	if chainIDStr == "" {
		fmt.Fprintln(os.Stderr, "error: CHAIN_ID is required")
		os.Exit(1)
	}
	offchainBaseURL := os.Getenv("OFFCHAIN_BASE_URL")
	if offchainBaseURL == "" {
		fmt.Fprintln(os.Stderr, "error: OFFCHAIN_BASE_URL is required")
		os.Exit(1)
	}
	jobContractAddr := os.Getenv("JOB_CONTRACT_ADDRESS")
	if jobContractAddr == "" {
		fmt.Fprintln(os.Stderr, "error: JOB_CONTRACT_ADDRESS is required")
		os.Exit(1)
	}
	registryContractAddr := os.Getenv("REGISTRY_CONTRACT_ADDRESS")
	if registryContractAddr == "" {
		fmt.Fprintln(os.Stderr, "error: REGISTRY_CONTRACT_ADDRESS is required")
		os.Exit(1)
	}

	pollInterval := envOrDefault("AGENT_POLL_INTERVAL", "10s")
	port := envOrDefault("AGENT_PORT", "9105")

	agentID, ok := new(big.Int).SetString(agentIDStr, 0)
	if !ok {
		agentID = nil // derive from wallet below
	}
	chainID, ok := new(big.Int).SetString(chainIDStr, 10)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: invalid CHAIN_ID: %s\n", chainIDStr)
		os.Exit(1)
	}
	pollDur, err := time.ParseDuration(pollInterval)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid AGENT_POLL_INTERVAL: %v\n", err)
		os.Exit(1)
	}

	// Initialize minimal logger config. LOG_PATH overrides the docker
	// default (/app/logs) for local runs.
	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		logPath = "/app/logs"
	}
	config.Cfg = &config.Config{
		Server: config.ServerConfig{LogPath: logPath},
	}
	logger.InitLogger()
	logger.Info("starting worker agent",
		logger.String("agent_id", agentIDStr),
		logger.String("port", port),
		logger.String("rpc_url", rpcURL),
		logger.String("offchain_url", offchainBaseURL),
		logger.String("poll_interval", pollInterval))

	// Parse private key and derive address.
	agentPrivKey = strings.TrimPrefix(agentPrivKey, "0x")
	keyBytes, err := hex.DecodeString(agentPrivKey)
	if err != nil {
		logger.Fatal("decode private key", logger.Error(err))
	}
	privKey, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		logger.Fatal("parse private key", logger.Error(err))
	}
	pubKey, ok := privKey.Public().(*ecdsa.PublicKey)
	if !ok {
		logger.Fatal("invalid public key type")
	}
	fromAddr := crypto.PubkeyToAddress(*pubKey)
	logger.Info("agent address derived", logger.String("address", fromAddr.Hex()))

	// Derive the agent id from the wallet: agentId = uint256(owner address).
	// AGENT_ID (if set) is only sanity-checked against it.
	derivedAgentID := new(big.Int).SetBytes(fromAddr[:])
	if agentID == nil {
		agentID = derivedAgentID
		logger.Info("agent id derived from wallet", logger.String("agent_id", agentID.String()))
	} else if agentID.Cmp(derivedAgentID) != 0 {
		logger.Warn("AGENT_ID does not match wallet; using derived id",
			logger.String("configured", agentID.String()),
			logger.String("derived", derivedAgentID.String()))
		agentID = derivedAgentID
	}

	// Connect to Ethereum RPC.
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		logger.Fatal("dial rpc", logger.Error(err))
	}
	defer client.Close()

	// Build transactOpts for on-chain writes.
	auth, err := chainbinding.NewTransactor(chainbinding.TransactorConfig{
		PrivateKeyHex: agentPrivKey,
		ChainID:       chainID,
	}, client)
	if err != nil {
		logger.Fatal("build transactor", logger.Error(err))
	}
	// Set gas defaults for Monad testnet (EIP-1559: tip + fee cap, no
	// legacy gasPrice — setting both makes go-ethereum reject the tx).
	auth.GasLimit = 300_000
	auth.GasPrice = nil // NewKeyedTransactorWithChainID defaults GasPrice to 1 GWei; clear it for EIP-1559 fields
	gasTipCap, err := client.SuggestGasTipCap(context.Background())
	if err != nil {
		gasTipCap = big.NewInt(1_000_000_000)
	}
	gasFeeCap, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		gasFeeCap = big.NewInt(10_000_000_000)
	}
	auth.GasTipCap = gasTipCap
	auth.GasFeeCap = gasFeeCap

	// Bind to job contract and registry contract.
	jobAddr := common.HexToAddress(jobContractAddr)
	jobContract, err := chainbinding.NewBoundContract(jobAddr, chainbinding.JobABI(), client)
	if err != nil {
		logger.Fatal("bind job contract", logger.Error(err))
	}

	regAddr := common.HexToAddress(registryContractAddr)
	regContract, err := chainbinding.NewBoundContract(regAddr, chainbinding.RegistryABI(), client)
	if err != nil {
		logger.Fatal("bind registry contract", logger.Error(err))
	}

	// Start health HTTP server.
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","agentId":%s}`, agentIDStr)
	})
	healthSrv := &http.Server{
		Addr:    ":" + port,
		Handler: healthMux,
	}
	go func() {
		logger.Info("health server starting", logger.String("port", port))
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("health server", logger.Error(err))
		}
	}()

	// Graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down")
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		healthSrv.Shutdown(shutdownCtx)
		client.Close()
	}()

	// Track seen job IDs to avoid duplicate processing.
	var seenMu sync.Mutex
	seen := make(map[string]bool)

	// Incremental cursor: per-port file under LOG_PATH's parent /tmp.
	// The cursor is the max DB event id consumed; on restart the worker
	// resumes from there instead of re-scanning the whole history.
	cursorPath := "/tmp/worker-cursor-" + port + ".txt"

	logger.Info("starting polling loop", logger.Duration("interval", pollDur))
	ticker := time.NewTicker(pollDur)
	defer ticker.Stop()

	// Run immediately on start, then on ticker.
	processJobs(ctx, agentID, auth, fromAddr, jobAddr, regAddr, jobContract, regContract, offchainBaseURL, client, seen, &seenMu, cursorPath)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processJobs(ctx, agentID, auth, fromAddr, jobAddr, regAddr, jobContract, regContract, offchainBaseURL, client, seen, &seenMu, cursorPath)
		}
	}
}

// processJobs fetches new PRISM_JOB_CREATED events (incrementally from the
// cursor) and attempts to grab each.
func processJobs(
	ctx context.Context,
	agentID *big.Int,
	auth *bind.TransactOpts,
	fromAddr common.Address,
	jobAddr, regAddr common.Address,
	jobContract, regContract *chainbinding.BoundContract,
	offchainBaseURL string,
	client *ethclient.Client,
	seen map[string]bool,
	seenMu *sync.Mutex,
	cursorPath string,
) {
	cursor := loadCursor(cursorPath)
	// Listen for PRISM_JOB_FUNDED: a job becomes grabbable only after it is
	// funded. CREATED events arrive before funding (state=0), so checking
	// them races the fund tx and misreports "not grabbable".
	events, err := fetchPrismEvents(ctx, offchainBaseURL, "PRISM_JOB_FUNDED", cursor, 1, 100)
	if err != nil {
		logger.Warn("fetch events failed", logger.Error(err))
		return
	}

	maxID := cursor
	for _, ev := range events {
		if ev.ID > maxID {
			maxID = ev.ID
		}
		// Backend filters by eventType; double-check for safety.
		if ev.EventType != "PRISM_JOB_FUNDED" {
			continue
		}
		seenMu.Lock()
		if seen[ev.JobID] {
			seenMu.Unlock()
			continue
		}
		// Mark as seen immediately to avoid duplicate processing.
		seen[ev.JobID] = true
		seenMu.Unlock()

		jobID := new(big.Int)
		// job_id comes back as 0x-prefixed hex; base 0 auto-detects it.
		if _, ok := jobID.SetString(ev.JobID, 0); !ok {
			logger.Warn("parse job ID", logger.String("job_id", ev.JobID))
			continue
		}

		logger.Info("inspecting job",
			logger.String("agent_id", agentID.String()),
			logger.String("job_id", ev.JobID))

		// Step 1: Check job state (must be Funded = 1).
		state, err := callGetJobState(ctx, jobContract, jobID)
		if err != nil {
			logger.Warn("getJobState failed",
				logger.String("job_id", ev.JobID),
				logger.Error(err))
			continue
		}
		if state != JobStateFunded {
			reportGrabAttempt(ctx, offchainBaseURL, agentID, jobID, false,
				fmt.Sprintf("任务不在可抢状态（state=%d：可能已被抢/已过期）", state))
			logger.Debug("job not funded, skipping",
				logger.String("job_id", ev.JobID),
				logger.Int("state", state))
			continue
		}

		// Step 2: Check provider reputation.
		score, err := callGetScore(ctx, regContract, agentID)
		if err != nil {
			logger.Warn("getScore failed",
				logger.String("agent_id", agentID.String()),
				logger.Error(err))
			continue
		}

		// Step 3: Get minProviderReputation from job state.
		minRep, err := callGetMinReputation(ctx, jobContract, jobID)
		if err != nil {
			logger.Warn("getMinReputation failed",
				logger.String("job_id", ev.JobID),
				logger.Error(err))
			continue
		}

		if score.Cmp(minRep) < 0 {
			reason := fmt.Sprintf("声誉不足（我的 %.2f < 要求 %.2f）", scoreToFloat(score), scoreToFloat(minRep))
			reportGrabAttempt(ctx, offchainBaseURL, agentID, jobID, false, reason)
			logger.Info("reputation too low to grab job",
				logger.String("job_id", ev.JobID),
				logger.String("score", score.String()),
				logger.String("min_required", minRep.String()))
			continue
		}

		// Step 3.5: Precheck grabJob via eth_call (no gas, no tx) so failures
		// are caught and reported before spending anything. From must be the
		// agent wallet, otherwise the owner check rejects the simulation.
		if err := precheckGrab(ctx, jobContract, fromAddr, jobID, agentID); err != nil {
			reason := parseGrabErr(err)
			reportGrabAttempt(ctx, offchainBaseURL, agentID, jobID, false, reason)
			logger.Warn("grabJob precheck failed",
				logger.String("job_id", ev.JobID),
				logger.String("reason", reason))
			continue
		}

		// Step 4: Grab the job.
		logger.Info("attempting to grab job",
			logger.String("job_id", ev.JobID),
			logger.String("score", score.String()))

		grabTxHash, err := callGrabJob(ctx, jobContract, auth, jobID, agentID)
		if err != nil {
			reportGrabAttempt(ctx, offchainBaseURL, agentID, jobID, false, parseGrabErr(err))
			logger.Warn("grabJob failed",
				logger.String("job_id", ev.JobID),
				logger.Error(err))
			continue
		}

		// Wait for the grab tx receipt to confirm success.
		receipt, err := waitForReceipt(ctx, client, grabTxHash)
		if err != nil || receipt == nil || receipt.Status != 1 {
			reportGrabAttempt(ctx, offchainBaseURL, agentID, jobID, false,
				"交易失败（可能被其他 agent 抢先）")
			logger.Warn("grab tx not confirmed",
				logger.String("job_id", ev.JobID),
				logger.Error(err))
			continue
		}
		reportGrabAttempt(ctx, offchainBaseURL, agentID, jobID, true, "")
		logger.Info("successfully grabbed job",
			logger.String("job_id", ev.JobID),
			logger.String("agent_id", agentID.String()))

		// Step 5: Random delay before submitting (1-3 seconds).
		delay := time.Duration(1+rand.Intn(3)) * time.Second
		logger.Info("waiting before submit",
			logger.String("job_id", ev.JobID),
			logger.Duration("delay", delay))

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		// Step 6: Submit mock deliverable.
		deliverableHash := crypto.Keccak256Hash([]byte("mock-audit-report-" + ev.JobID))
		proofHash := crypto.Keccak256Hash([]byte("mock-proof-" + ev.JobID))

		if err := callSubmit(ctx, jobContract, auth, jobID, deliverableHash, proofHash); err != nil {
			logger.Warn("submit failed",
				logger.String("job_id", ev.JobID),
				logger.Error(err))
			continue
		}

		logger.Info("successfully submitted deliverable",
			logger.String("job_id", ev.JobID),
			logger.String("deliverable_hash", deliverableHash.Hex()),
			logger.String("proof_hash", proofHash.Hex()))
	}

	// Advance the cursor past everything we just saw, even failed grabs:
	// job state checks are idempotent and re-inspecting old events is waste.
	if maxID > cursor {
		saveCursor(cursorPath, maxID)
	}
}

// loadCursor reads the last consumed event id from the cursor file.
func loadCursor(path string) uint64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var id uint64
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &id); err != nil {
		return 0
	}
	return id
}

// saveCursor persists the consumed event id to the cursor file.
func saveCursor(path string, id uint64) {
	_ = os.WriteFile(path, []byte(fmt.Sprintf("%d", id)), 0o644)
}

// fetchPrismEvents calls the offchain API to get prism events.
// chainName is required by the backend (400 without it); minID switches to
// incremental cursor mode (id > minID, ascending); the API token is sent as
// X-API-TOKEN when OFFCHAIN_API_TOKEN is set.
func fetchPrismEvents(ctx context.Context, baseURL, eventType string, minID uint64, page, size int) ([]PrismEventItem, error) {
	url := fmt.Sprintf("%s/api/v1/prismsettle/events?chainName=monad_testnet&eventType=%s&minId=%d&page=%d&size=%d",
		strings.TrimRight(baseURL, "/"), eventType, minID, page, size)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if tok := os.Getenv("OFFCHAIN_API_TOKEN"); tok != "" {
		req.Header.Set("X-API-TOKEN", tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result PrismEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Data.List, nil
}

// callGetJobState calls getJobState on the job contract and returns the state.
func callGetJobState(ctx context.Context, contract *chainbinding.BoundContract, jobID *big.Int) (int, error) {
	bc := contract.Raw()
	var outs []interface{}
	if err := bc.Call(&bind.CallOpts{Context: ctx}, &outs, "getJobState", jobID); err != nil {
		return 0, fmt.Errorf("getJobState call: %w", err)
	}
	if len(outs) < 1 {
		return 0, fmt.Errorf("getJobState: unexpected output length %d", len(outs))
	}
	state, ok := outs[0].(uint8)
	if !ok {
		return 0, fmt.Errorf("getJobState: unexpected state type %T", outs[0])
	}
	return int(state), nil
}

// callGetMinReputation returns the minProviderReputation field from getJobState.
func callGetMinReputation(ctx context.Context, contract *chainbinding.BoundContract, jobID *big.Int) (*big.Int, error) {
	bc := contract.Raw()
	var outs []interface{}
	if err := bc.Call(&bind.CallOpts{Context: ctx}, &outs, "getJobState", jobID); err != nil {
		return nil, fmt.Errorf("getJobState call: %w", err)
	}
	// getJobState returns 10 fields; index 8 = minProviderReputation (uint96).
	if len(outs) < 9 {
		return nil, fmt.Errorf("getJobState: unexpected output length %d", len(outs))
	}
	minRep, ok := outs[8].(uint64) // uint96 is returned as uint64 by go-ethereum
	if !ok {
		// Try uint64 if it's a *big.Int.
		if bi, ok := outs[8].(*big.Int); ok {
			return bi, nil
		}
		return nil, fmt.Errorf("getJobState: unexpected minRep type %T", outs[8])
	}
	return new(big.Int).SetUint64(minRep), nil
}

// callGetScore calls getScore on the registry contract.
func callGetScore(ctx context.Context, contract *chainbinding.BoundContract, agentID *big.Int) (*big.Int, error) {
	bc := contract.Raw()
	var outs []interface{}
	if err := bc.Call(&bind.CallOpts{Context: ctx}, &outs, "getScore", agentID); err != nil {
		return nil, fmt.Errorf("getScore call: %w", err)
	}
	if len(outs) < 1 {
		return nil, fmt.Errorf("getScore: unexpected output length %d", len(outs))
	}
	switch v := outs[0].(type) {
	case uint64:
		return new(big.Int).SetUint64(v), nil
	case *big.Int:
		return v, nil
	default:
		return nil, fmt.Errorf("getScore: unexpected type %T", outs[0])
	}
}

// callGrabJob calls grabJob(jobId, providerAgentId) on the job contract and
// returns the tx hash (empty on failure).
func callGrabJob(ctx context.Context, contract *chainbinding.BoundContract, auth *bind.TransactOpts, jobID, agentID *big.Int) (common.Hash, error) {
	bc := contract.Raw()
	tx, err := bc.Transact(auth, "grabJob", jobID, agentID)
	if err != nil {
		return common.Hash{}, fmt.Errorf("grabJob transact: %w", err)
	}
	logger.Info("grabJob tx sent", logger.String("tx_hash", tx.Hash().Hex()))
	return tx.Hash(), nil
}

// callSubmit calls submit(jobId, deliverableHash, proofHash) on the job contract.
func callSubmit(ctx context.Context, contract *chainbinding.BoundContract, auth *bind.TransactOpts, jobID *big.Int, deliverableHash, proofHash common.Hash) error {
	bc := contract.Raw()
	tx, err := bc.Transact(auth, "submit", jobID, deliverableHash, proofHash)
	if err != nil {
		return fmt.Errorf("submit transact: %w", err)
	}
	logger.Info("submit tx sent", logger.String("tx_hash", tx.Hash().Hex()))
	return nil
}

// precheckGrab simulates grabJob via eth_call (no gas, no tx) so failures are
// caught before spending anything. A non-nil error carries the revert reason.
// from must be the agent's wallet so owner checks pass for legitimate grabs.
func precheckGrab(ctx context.Context, contract *chainbinding.BoundContract, from common.Address, jobID, agentID *big.Int) error {
	bc := contract.Raw()
	var outs []interface{}
	if err := bc.Call(&bind.CallOpts{Context: ctx, From: from}, &outs, "grabJob", jobID, agentID); err != nil {
		return err
	}
	return nil
}

// parseGrabErr maps a grabJob error to a human-readable (Chinese) reason.
func parseGrabErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "already assigned"):
		return "已被其他 agent 抢走"
	case strings.Contains(s, "bad state"):
		return "任务状态已变化（可能被抢或已过期）"
	case strings.Contains(s, "not owner"):
		return "agentId 与钱包不匹配（归属校验失败）"
	case strings.Contains(s, "reputation too low"):
		return "声誉不足"
	case strings.Contains(s, "self-dealing"):
		return "不能抢自己发布的任务"
	default:
		return s
	}
}

// scoreToFloat converts a 1e18-scaled score to a human-readable float.
func scoreToFloat(v *big.Int) float64 {
	f, _ := new(big.Float).Quo(new(big.Float).SetInt(v), big.NewFloat(1e18)).Float64()
	return f
}

// waitForReceipt polls until the tx is mined; returns the receipt.
func waitForReceipt(ctx context.Context, client *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	timeout := time.After(60 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("wait receipt timeout")
		default:
		}
		receipt, err := client.TransactionReceipt(ctx, txHash)
		if err == nil && receipt != nil {
			return receipt, nil
		}
		time.Sleep(2 * time.Second)
	}
}

// reportGrabAttempt posts a grab attempt (success/failure + reason) to the
// offchain API so operators can see why their agent lost a job competition.
func reportGrabAttempt(ctx context.Context, baseURL string, agentID, jobID *big.Int, success bool, reason string) {
	body, err := json.Marshal(map[string]interface{}{
		"chain_name": "monad_testnet",
		"agent_id":   "0x" + fmt.Sprintf("%064x", agentID),
		"job_id":     "0x" + fmt.Sprintf("%064x", jobID),
		"success":    success,
		"reason":     reason,
	})
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(baseURL, "/")+"/api/v1/prismsettle/grab-attempts",
		bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := os.Getenv("OFFCHAIN_API_TOKEN"); tok != "" {
		req.Header.Set("X-API-TOKEN", tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Warn("report grab attempt failed", logger.Error(err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logger.Warn("report grab attempt: bad status", logger.Int("status", resp.StatusCode))
	}
}
