package rpc

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zane/web3-offchain/pkg/config"
	"github.com/zane/web3-offchain/pkg/logger"
)

// healthCacheTTL is how long a successful health check is trusted before
// re-validating. Without this, every GetClient call issues an extra
// BlockNumber round-trip, doubling RPC cost and defeating round-robin perf.
const healthCacheTTL = 5 * time.Second

// nodeHealth caches the last health-check result for a single RPC node.
type nodeHealth struct {
	mu        sync.Mutex
	healthy   bool
	checkedAt time.Time
}

// RPCClient manages multiple RPC nodes across multiple blockchains
// It provides automatic failover, round-robin load balancing, and health checking
type RPCClient struct {
	// chainNodes maps chain name to a list of healthy RPC clients
	chainNodes map[string][]*ethclient.Client
	// chainIdx stores atomic round-robin index for each chain
	chainIdx map[string]*uint32
	// health[key(chain,nodeIdx)] caches recent health-check results so we
	// don't pay a BlockNumber round-trip on every GetClient call.
	health sync.Map // map[string]*nodeHealth
}

// NewRPCClient initializes a multi-chain, multi-node RPC client manager
// It validates node availability during startup and skips unhealthy endpoints
func NewRPCClient() (*RPCClient, error) {
	chainNodes := make(map[string][]*ethclient.Client)
	chainIdx := make(map[string]*uint32)

	// Initialize RPC clients for all configured chains
	for _, chain := range config.Cfg.Web3.Chains {
		chainName := chain.ChainName
		rpcURLs := chain.RPCUrls

		for _, rpcURL := range rpcURLs {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			// Dial RPC endpoint
			rpcClient, err := rpc.DialOptions(ctx, rpcURL)
			if err != nil {
				logger.Warn("failed to dial RPC node",
					logger.String("chain", chainName),
					logger.String("url", rpcURL),
					logger.Error(err),
				)
				cancel()
				continue
			}

			// Wrap with ethclient and validate connectivity
			client := ethclient.NewClient(rpcClient)
			_, err = client.BlockNumber(ctx)
			cancel()

			if err != nil {
				logger.Warn("RPC node health check failed",
					logger.String("chain", chainName),
					logger.String("url", rpcURL),
					logger.Error(err),
				)
				client.Close()
				continue
			}

			// Add healthy node to the pool
			chainNodes[chainName] = append(chainNodes[chainName], client)
			chainIdx[chainName] = new(uint32)
			logger.Info("RPC node added successfully",
				logger.String("chain", chainName),
				logger.String("url", rpcURL),
			)
		}
	}

	// Reject startup if no healthy nodes exist for any chain
	if len(chainNodes) == 0 {
		return nil, fmt.Errorf("no healthy RPC nodes available for any chain")
	}

	// Warn about chains with no healthy nodes
	for _, chain := range config.Cfg.Web3.Chains {
		if _, ok := chainNodes[chain.ChainName]; !ok {
			logger.Warn("no healthy RPC nodes for chain", logger.String("chain", chain.ChainName))
		}
	}

	return &RPCClient{
		chainNodes: chainNodes,
		chainIdx:   chainIdx,
	}, nil
}

// GetClient returns a healthy RPC client for the given chain using round-robin selection.
//
// Health is verified by a BlockNumber probe, but recent successful probes are
// cached for healthCacheTTL so we don't pay the extra round-trip on every call.
// On a probe failure the cached state is flipped to unhealthy and we failover
// to the next node.
func (r *RPCClient) GetClient(chainName string) (*ethclient.Client, error) {
	nodes, ok := r.chainNodes[chainName]
	if !ok || len(nodes) == 0 {
		return nil, fmt.Errorf("no RPC nodes configured for chain: %s", chainName)
	}

	nodeCount := uint32(len(nodes))
	// Atomic increment for thread-safe round-robin
	currentIdx := atomic.AddUint32(r.chainIdx[chainName], 1) % nodeCount

	// Try each node starting from the current index
	for i := uint32(0); i < nodeCount; i++ {
		idx := (currentIdx + i) % nodeCount
		client := nodes[idx]

		if r.isHealthy(chainName, idx, client) {
			return client, nil
		}

		logger.Warn("RPC node unhealthy, failing over",
			logger.String("chain", chainName),
			logger.Uint32("index", idx),
		)
	}

	// All nodes failed
	return nil, fmt.Errorf("all RPC nodes for chain %s are unavailable", chainName)
}

// isHealthy returns true if the node is healthy, performing a BlockNumber probe
// only when the cached result is older than healthCacheTTL.
func (r *RPCClient) isHealthy(chainName string, idx uint32, client *ethclient.Client) bool {
	key := fmt.Sprintf("%s:%d", chainName, idx)
	hAny, _ := r.health.LoadOrStore(key, &nodeHealth{})
	h := hAny.(*nodeHealth)

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.healthy && time.Since(h.checkedAt) < healthCacheTTL {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := client.BlockNumber(ctx)
	cancel()

	now := time.Now()
	h.healthy = err == nil
	h.checkedAt = now

	if h.healthy {
		logger.Debug("selected healthy RPC node",
			logger.String("chain", chainName),
			logger.Uint32("index", idx),
		)
	}
	return h.healthy
}

// GetBlockWithRetry retrieves a block by number with timeout and failover support
func (r *RPCClient) GetBlockWithRetry(
	ctx context.Context,
	chainName string,
	blockNumber uint64,
) (*types.Block, error) {
	client, err := r.GetClient(chainName)
	if err != nil {
		return nil, err
	}

	// Enforce per-request timeout to prevent hanging
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	return client.BlockByNumber(reqCtx, big.NewInt(int64(blockNumber)))
}

// Close gracefully shuts down all RPC clients and releases connections
func (r *RPCClient) Close() {
	for chainName, nodes := range r.chainNodes {
		for idx, client := range nodes {
			if client != nil {
				client.Close()
				logger.Debug("closed RPC client connection",
					logger.String("chain", chainName),
					logger.Int("node_index", idx),
				)
			}
		}
	}
	logger.Info("all RPC client connections closed successfully")
}
