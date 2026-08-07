// Package main implements a smart-contract auditor worker agent for the
// PrismSettle demo. It polls the offchain REST API for available jobs,
// competes by reputation to grab them, and submits mock audit deliverables.
package main

import (
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
type PrismEventItem struct {
	EventType string `json:"eventType"`
	JobID     string `json:"jobId"`
	From      string `json:"from"`
	TxHash    string `json:"txHash"`
	LogIndex  uint   `json:"logIndex"`
	RawData   string `json:"rawData"`
	ChainName string `json:"chainName"`
	CreatedAt string `json:"createdAt"`
}

// PrismEventsResponse wraps the paginated event list.
type PrismEventsResponse struct {
	Data     []PrismEventItem `json:"data"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
}

func main() {
	agentIDStr := os.Getenv("AGENT_ID")
	if agentIDStr == "" {
		fmt.Fprintln(os.Stderr, "error: AGENT_ID is required")
		os.Exit(1)
	}
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
		fmt.Fprintf(os.Stderr, "error: invalid AGENT_ID: %s\n", agentIDStr)
		os.Exit(1)
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

	// Initialize minimal logger config.
	config.Cfg = &config.Config{
		Server: config.ServerConfig{LogPath: "/app/logs"},
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
	// Set gas defaults for Monad testnet.
	auth.GasLimit = 300_000
	gasTipCap, err := client.SuggestGasTipCap(context.Background())
	if err != nil {
		gasTipCap = big.NewInt(1_000_000_000)
	}
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		gasPrice = big.NewInt(10_000_000_000)
	}
	auth.GasTipCap = gasTipCap
	auth.GasPrice = gasPrice

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

	logger.Info("starting polling loop", logger.Duration("interval", pollDur))
	ticker := time.NewTicker(pollDur)
	defer ticker.Stop()

	// Run immediately on start, then on ticker.
	processJobs(ctx, agentID, auth, fromAddr, jobAddr, regAddr, jobContract, regContract, offchainBaseURL, client, seen, &seenMu)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processJobs(ctx, agentID, auth, fromAddr, jobAddr, regAddr, jobContract, regContract, offchainBaseURL, client, seen, &seenMu)
		}
	}
}

// processJobs fetches new PRISM_JOB_CREATED events and attempts to grab each.
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
) {
	events, err := fetchPrismEvents(ctx, offchainBaseURL, "PRISM_JOB_CREATED", 1, 20)
	if err != nil {
		logger.Warn("fetch events failed", logger.Error(err))
		return
	}

	for _, ev := range events {
		seenMu.Lock()
		if seen[ev.JobID] {
			seenMu.Unlock()
			continue
		}
		// Mark as seen immediately to avoid duplicate processing.
		seen[ev.JobID] = true
		seenMu.Unlock()

		jobID := new(big.Int)
		if _, ok := jobID.SetString(ev.JobID, 10); !ok {
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
			logger.Info("reputation too low to grab job",
				logger.String("job_id", ev.JobID),
				logger.String("score", score.String()),
				logger.String("min_required", minRep.String()))
			continue
		}

		// Step 4: Grab the job.
		logger.Info("attempting to grab job",
			logger.String("job_id", ev.JobID),
			logger.String("score", score.String()))

		if err := callGrabJob(ctx, jobContract, auth, jobID, agentID); err != nil {
			logger.Warn("grabJob failed",
				logger.String("job_id", ev.JobID),
				logger.Error(err))
			continue
		}

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
}

// fetchPrismEvents calls the offchain API to get prism events.
func fetchPrismEvents(ctx context.Context, baseURL, eventType string, page, size int) ([]PrismEventItem, error) {
	url := fmt.Sprintf("%s/api/v1/prismsettle/events?eventType=%s&page=%d&size=%d",
		strings.TrimRight(baseURL, "/"), eventType, page, size)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
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

	return result.Data, nil
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

// callGrabJob calls grabJob(jobId, providerAgentId) on the job contract.
func callGrabJob(ctx context.Context, contract *chainbinding.BoundContract, auth *bind.TransactOpts, jobID, agentID *big.Int) error {
	bc := contract.Raw()
	tx, err := bc.Transact(auth, "grabJob", jobID, agentID)
	if err != nil {
		return fmt.Errorf("grabJob transact: %w", err)
	}
	logger.Info("grabJob tx sent", logger.String("tx_hash", tx.Hash().Hex()))
	return nil
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
