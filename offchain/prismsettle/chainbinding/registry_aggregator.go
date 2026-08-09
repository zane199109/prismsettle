package chainbinding

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zane/web3-offchain/internal/repository"
	"github.com/zane/web3-offchain/pkg/logger"
	"github.com/zane/web3-offchain/prismsettle/keeper"
)

// RegistryAggregatorConfig configures the on-chain RegistryAggregator.
type RegistryAggregatorConfig struct {
	ChainName    string // e.g. "monad_testnet"
	RegContract  string // PrismSettleRegistry contract address (lowercase)
	HookContract string // ArbitrationHook contract address (lowercase, unused for now)
}

// RegistryAggregatorBinding implements keeper.RegistryAggregator by combining
// on-chain Registry calls (aggregateEpoch, getScore, getValidationCount,
// agents) with offchain agent_registry table scans.
//
// Design notes (contract reality vs. Keeper interface):
//
//  1. PrismSettleRegistry has NO `tickDecay` function. Inactivity decay is
//     applied lazily inside `aggregateEpoch` via `applyDecay`. So TickDecay
//     is implemented as a call to `aggregateEpoch` — the contract applies the
//     decay if the EPOCH interval has elapsed since lastAggregate.
//
//  2. PrismSettleRegistry has NO `PendingAgents` / `InactiveAgents` view
//     functions. We scan the offchain agent_registry table (populated from
//     AgentRegistered events) and filter via per-agent on-chain views:
//     - PendingAgents:   getValidationCount(agentId) > 0
//     - InactiveAgents:  agents(agentId).lastActivity < now - inactiveAge
//
//     This is O(N) view calls per tick, acceptable for testnet (<100 agents).
//     For mainnet, add an offchain "pending set" maintained from
//     ValidationSubmitted/Aggregated event diff (Phase 10 optimization).
type RegistryAggregatorBinding struct {
	cfg      RegistryAggregatorConfig
	contract boundContract
	auth     *bind.TransactOpts
	agentDB  *repository.AgentRegistryRepository
}

// NewRegistryAggregator builds a RegistryAggregator bound to the
// PrismSettleRegistry contract.
//
// auth is required for AggregateEpoch / TickDecay (write operations). For
// read-only use (GetScore only), pass nil — the binding will only be able
// to call view functions.
func NewRegistryAggregator(
	cfg RegistryAggregatorConfig,
	regAddr common.Address,
	client *ethclient.Client,
	auth *bind.TransactOpts,
	agentDB *repository.AgentRegistryRepository,
) (*RegistryAggregatorBinding, error) {
	bc, err := newBoundContract(regAddr, registryABI, client)
	if err != nil {
		return nil, fmt.Errorf("registry aggregator: %w", err)
	}
	return &RegistryAggregatorBinding{
		cfg:      cfg,
		contract: bc,
		auth:     auth,
		agentDB:  agentDB,
	}, nil
}

// Compile-time assertion.
var _ keeper.RegistryAggregator = (*RegistryAggregatorBinding)(nil)

// PendingAgents returns agentIds that have unaggregated validation records.
// Scans the offchain agent_registry table and checks getValidationCount
// on-chain for each agent.
func (r *RegistryAggregatorBinding) PendingAgents(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}
	agentIDs, err := r.listAgentIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	out := make([]string, 0, limit)
	for _, idHex := range agentIDs {
		if len(out) >= limit {
			break
		}
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		count, err := r.getValidationCount(ctx, idHex)
		if err != nil {
			logger.Warn("aggregator: getValidationCount failed",
				logger.String("agent", idHex), logger.Error(err))
			continue
		}
		if count > 0 {
			out = append(out, idHex)
		}
	}
	return out, nil
}

// AggregateEpoch calls Registry.aggregateEpoch(agentId). Returns the tx hash.
func (r *RegistryAggregatorBinding) AggregateEpoch(ctx context.Context, agentID string) (string, error) {
	if r.auth == nil {
		return "", fmt.Errorf("aggregator: transactor not configured (auth is nil)")
	}
	id, err := parseAgentID(agentID)
	if err != nil {
		return "", fmt.Errorf("aggregator: parse agent id: %w", err)
	}
	tx, err := r.contract.bind.Transact(r.auth, "aggregateEpoch", id)
	if err != nil {
		return "", fmt.Errorf("aggregate epoch transact: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// SetAggregatedScore calls Registry.setAggregatedScore(agentId, newScore)
// with an offchain-computed score (EMA + penalty + decay).
func (r *RegistryAggregatorBinding) SetAggregatedScore(ctx context.Context, agentID string, newScore uint64) (string, error) {
	if r.auth == nil {
		return "", fmt.Errorf("aggregator: transactor not configured (auth is nil)")
	}
	id, err := parseAgentID(agentID)
	if err != nil {
		return "", fmt.Errorf("aggregator: parse agent id: %w", err)
	}
	tx, err := r.contract.bind.Transact(r.auth, "setAggregatedScore", id, newScore)
	if err != nil {
		return "", fmt.Errorf("set aggregated score transact: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// InactiveAgents returns agentIds whose lastActivity < now - inactiveAge.
// Reads the `agents(agentId)` view to get lastActivity for each registered
// agent.
func (r *RegistryAggregatorBinding) InactiveAgents(ctx context.Context, inactiveAge time.Duration, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}
	agentIDs, err := r.listAgentIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	threshold := time.Now().Add(-inactiveAge).Unix()
	out := make([]string, 0, limit)
	for _, idHex := range agentIDs {
		if len(out) >= limit {
			break
		}
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		lastActivity, err := r.getLastActivity(ctx, idHex)
		if err != nil {
			logger.Warn("aggregator: getLastActivity failed",
				logger.String("agent", idHex), logger.Error(err))
			continue
		}
		if lastActivity == 0 {
			continue // never active, skip
		}
		if int64(lastActivity) < threshold {
			out = append(out, idHex)
		}
	}
	return out, nil
}

// TickDecay calls Registry.aggregateEpoch(agentId).
//
// IMPORTANT: PrismSettleRegistry has no separate `tickDecay` function.
// Inactivity decay is applied inside `aggregateEpoch` via `applyDecay`.
// The EPOCH interval guard (require lastAggregate + EPOCH <= now) prevents
// calling this more than once per EPOCH per agent, which is acceptable for
// the Keeper's 1h scan cadence.
//
// If aggregateEpoch reverts with "epoch not due", the agent's lastAggregate
// is recent and decay has already been applied — treat as success (no-op).
func (r *RegistryAggregatorBinding) TickDecay(ctx context.Context, agentID string) (string, error) {
	if r.auth == nil {
		return "", fmt.Errorf("aggregator: transactor not configured (auth is nil)")
	}
	id, err := parseAgentID(agentID)
	if err != nil {
		return "", fmt.Errorf("aggregator: parse agent id: %w", err)
	}
	tx, err := r.contract.bind.Transact(r.auth, "aggregateEpoch", id)
	if err != nil {
		// If the revert is "epoch not due", decay was already applied —
		// return empty tx hash instead of erroring so the Keeper doesn't
		// log warnings for agents that just had their epoch aggregated.
		//
		// P3-N note: we use strings.Contains here instead of errors.Is
		// because EVM revert reasons are plain strings embedded in the
		// error message by go-ethereum's Transact wrapper. There is no
		// sentinel error type to compare against — the revert reason is
		// defined in Solidity as require(block.number >= ..., "epoch not due")
		// and surfaces as part of the error string.
		if strings.Contains(err.Error(), "epoch not due") {
			logger.Debug("aggregator: epoch not due, decay already applied",
				logger.String("agent", agentID))
			return "", nil
		}
		return "", fmt.Errorf("tick decay transact: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// GetScore reads the live aggregated score from the Registry contract.
// Used by ChainEventSource.CurrentScore as the ScoreFetcher implementation.
func (r *RegistryAggregatorBinding) GetScore(ctx context.Context, agentID *big.Int) (uint64, error) {
	var outs []interface{}
	if err := r.contract.bind.Call(&bind.CallOpts{Context: ctx}, &outs, "getScore", agentID); err != nil {
		return 0, fmt.Errorf("get score call: %w", err)
	}
	if len(outs) == 0 {
		return 0, nil
	}
	score, ok := outs[0].(*big.Int)
	if !ok || score == nil {
		return 0, nil
	}
	return score.Uint64(), nil
}

// --- internal helpers ------------------------------------------------

// listAgentIDs returns all agentIds (hex uint256) from the offchain
// agent_registry table for the configured chain.
func (r *RegistryAggregatorBinding) listAgentIDs(ctx context.Context) ([]string, error) {
	records, err := r.agentDB.List(ctx, r.cfg.ChainName)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(records))
	for _, rec := range records {
		ids = append(ids, rec.AgentID)
	}
	return ids, nil
}

// getValidationCount calls Registry.getValidationCount(agentId) view.
func (r *RegistryAggregatorBinding) getValidationCount(ctx context.Context, agentIDHex string) (uint64, error) {
	id, err := parseAgentID(agentIDHex)
	if err != nil {
		return 0, err
	}
	var outs []interface{}
	if err := r.contract.bind.Call(&bind.CallOpts{Context: ctx}, &outs, "getValidationCount", id); err != nil {
		return 0, fmt.Errorf("getValidationCount call: %w", err)
	}
	if len(outs) == 0 {
		return 0, nil
	}
	count, ok := outs[0].(*big.Int)
	if !ok || count == nil {
		return 0, nil
	}
	return count.Uint64(), nil
}

// getLastActivity reads agents(agentId).lastActivity (field index 4 in the
// ABI return tuple).
func (r *RegistryAggregatorBinding) getLastActivity(ctx context.Context, agentIDHex string) (uint64, error) {
	id, err := parseAgentID(agentIDHex)
	if err != nil {
		return 0, err
	}
	var outs []interface{}
	if err := r.contract.bind.Call(&bind.CallOpts{Context: ctx}, &outs, "agents", id); err != nil {
		return 0, fmt.Errorf("agents call: %w", err)
	}
	// agents() returns 9 fields; field index 4 = lastActivity (uint64).
	if len(outs) < 5 {
		return 0, fmt.Errorf("agents call: unexpected output length %d", len(outs))
	}
	lastActivity, ok := outs[4].(uint64)
	if !ok {
		return 0, fmt.Errorf("agents call: lastActivity field has unexpected type %T", outs[4])
	}
	return lastActivity, nil
}

// parseAgentID converts a 0x-prefixed 32-byte hex string (parser storage
// convention for agentId) to *big.Int.
func parseAgentID(hex string) (*big.Int, error) {
	hex = strings.TrimSpace(hex)
	if !strings.HasPrefix(hex, "0x") {
		return nil, fmt.Errorf("agent id must be 0x-prefixed hex, got %q", hex)
	}
	id := new(big.Int)
	if _, ok := id.SetString(strings.TrimPrefix(hex, "0x"), 16); !ok {
		return nil, fmt.Errorf("invalid agent id hex: %q", hex)
	}
	return id, nil
}
