package service

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	errInvalidScoreID = errors.New("invalid agent id for score lookup")
	errNoScore        = errors.New("no score returned")
)

// scoreCacheTTL bounds how long a fetched score is reused. Scores change only
// via aggregateEpoch / seed, so 60s is safe and avoids hammering the RPC on
// every /agents poll (testnet RPC rate-limits heavily).
const scoreCacheTTL = 60 * time.Second

type scoreEntry struct {
	value     string
	fetchedAt time.Time
}

// ScoreFetcher resolves a live on-chain reputation score via eth_call to
// registry.getScore(uint256). Used by listAgents as a fallback for agents
// whose score has not been aggregated on-chain yet — seedAgent() writes
// aggregatedScore directly without emitting PRISM_AGGREGATED, so the DB has
// no event row for seeded agents even though the contract holds the value.
type ScoreFetcher struct {
	client   *ethclient.Client
	registry common.Address
	mu       sync.Mutex
	cache    map[string]scoreEntry
}

var getScoreCalldataPrefix = crypto.Keccak256([]byte("getScore(uint256)"))[:4]

// NewScoreFetcher dials the chain RPC and pins the registry contract address.
func NewScoreFetcher(rpcURL, registryAddr string) (*ScoreFetcher, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	return &ScoreFetcher{
		client:   client,
		registry: common.HexToAddress(registryAddr),
		cache:    make(map[string]scoreEntry),
	}, nil
}

// FetchScore returns the agent's aggregated score as a decimal string
// (1e18-scaled), or an error when the call fails (e.g. agent not registered).
// Results are cached for scoreCacheTTL to keep /agents fast under RPC limits.
func (f *ScoreFetcher) FetchScore(ctx context.Context, agentID string) (string, error) {
	f.mu.Lock()
	if e, ok := f.cache[agentID]; ok && time.Since(e.fetchedAt) < scoreCacheTTL {
		f.mu.Unlock()
		return e.value, nil
	}
	f.mu.Unlock()

	id, ok := new(big.Int).SetString(strings.TrimPrefix(agentID, "0x"), 16)
	if !ok {
		return "", errInvalidScoreID
	}
	calldata := append(append([]byte{}, getScoreCalldataPrefix...), common.LeftPadBytes(id.Bytes(), 32)...)
	out, err := f.client.CallContract(ctx, ethereum.CallMsg{
		To:   &f.registry,
		Data: calldata,
	}, nil)
	if err != nil {
		return "", err
	}
	if len(out) < 32 {
		return "", errNoScore
	}
	value := new(big.Int).SetBytes(out[:32]).String()

	f.mu.Lock()
	f.cache[agentID] = scoreEntry{value: value, fetchedAt: time.Now()}
	f.mu.Unlock()
	return value, nil
}

// Close releases the underlying RPC connection.
func (f *ScoreFetcher) Close() {
	if f.client != nil {
		f.client.Close()
	}
}
