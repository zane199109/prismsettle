package keeper

import (
	"context"
	"fmt"
	"time"

	"github.com/zane/web3-offchain/pkg/logger"
)

// HealthTracker is the minimal interface the Keeper needs from the Phase 9
// HealthTracker. Defining it here avoids a circular import (service depends
// on keeper indirectly via main.go wiring; keeper must not depend on service).
// The concrete *service.HealthTracker satisfies this interface.
type HealthTracker interface {
	TouchKeeper()
}

// Keeper is the Phase 6 background bot (SD §4.6). It does two things:
//
//  1. Epoch aggregation: every 30s, fetches up to maxBatch agents with
//     pending validations and calls Registry.aggregateEpoch(agentId) for each.
//     Batching + 30s cadence spreads gas cost and avoids gas spikes.
//
//  2. Inactive-agent scanner: every 1h, scans agents whose lastActivity is
//     older than 30d. For each, calls Registry.tickDecay(agentId) to apply
//     the 0.01e18/day inactivity penalty (SD §4.6.3). Pure lazy-decay would
//     never fire for inactive agents; the Keeper is the dual-path solution.
//
// The Keeper is intentionally stateless between ticks: it queries the chain
// for pending work each tick, so a restart loses nothing.
type Keeper struct {
	registry    RegistryAggregator
	health      HealthTracker // optional; nil = no health reporting
	pendingSize int           // max agents to aggregate per tick
	tickPeriod  time.Duration // default 30s
	scanPeriod  time.Duration // default 1h
	inactiveAge time.Duration // default 30d
}

// RegistryAggregator is the on-chain interface the Keeper needs.
type RegistryAggregator interface {
	// PendingAgents returns agentIds that have unaggregated validation records
	// (shardValidations[agentId].length > 0). limit caps the result.
	PendingAgents(ctx context.Context, limit int) ([]string, error)
	// AggregateEpoch calls Registry.aggregateEpoch(agentId).
	AggregateEpoch(ctx context.Context, agentID string) (string, error)
	// InactiveAgents returns agentIds whose lastActivity < now - inactiveAge.
	InactiveAgents(ctx context.Context, inactiveAge time.Duration, limit int) ([]string, error)
	// TickDecay calls Registry.tickDecay(agentId) (SD §4.6.3 lazy decay trigger).
	TickDecay(ctx context.Context, agentID string) (string, error)
}

// Config holds Keeper tuning knobs.
type Config struct {
	TickPeriod  time.Duration // default 30s
	ScanPeriod  time.Duration // default 1h
	InactiveAge time.Duration // default 30d
	BatchSize   int           // default 10
}

// NewKeeper builds a Keeper.
func NewKeeper(cfg Config, registry RegistryAggregator) (*Keeper, error) {
	if registry == nil {
		return nil, fmt.Errorf("keeper: registry is required")
	}
	if cfg.TickPeriod <= 0 {
		cfg.TickPeriod = 30 * time.Second
	}
	if cfg.ScanPeriod <= 0 {
		cfg.ScanPeriod = time.Hour
	}
	if cfg.InactiveAge <= 0 {
		cfg.InactiveAge = 30 * 24 * time.Hour
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 10
	}
	return &Keeper{
		registry:    registry,
		pendingSize: cfg.BatchSize,
		tickPeriod:  cfg.TickPeriod,
		scanPeriod:  cfg.ScanPeriod,
		inactiveAge: cfg.InactiveAge,
	}, nil
}

// SetHealthTracker wires the Phase 9 HealthTracker so the Keeper can report
// tick completion for the /health endpoint (NFR-OBS02). Optional: nil means
// health reporting is disabled. Must be called before Run.
func (k *Keeper) SetHealthTracker(h HealthTracker) {
	k.health = h
}

// Run starts the Keeper. Blocks until ctx is canceled.
func (k *Keeper) Run(ctx context.Context) error {
	logger.Info("keeper started",
		logger.String("tick", k.tickPeriod.String()),
		logger.String("scan", k.scanPeriod.String()),
		logger.String("inactive_age", k.inactiveAge.String()))

	tick := time.NewTicker(k.tickPeriod)
	defer tick.Stop()
	scan := time.NewTicker(k.scanPeriod)
	defer scan.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("keeper stopping")
			return ctx.Err()
		case <-tick.C:
			if err := k.aggregateBatch(ctx); err != nil {
				logger.Errorf("keeper aggregate batch failed", logger.Error(err))
			}
		case <-scan.C:
			if err := k.scanInactive(ctx); err != nil {
				logger.Errorf("keeper inactive scan failed", logger.Error(err))
			}
		}
	}
}

// aggregateBatch fetches pending agents and aggregates up to batchSize of them.
func (k *Keeper) aggregateBatch(ctx context.Context) error {
	agents, err := k.registry.PendingAgents(ctx, k.pendingSize)
	if err != nil {
		return fmt.Errorf("fetch pending agents: %w", err)
	}
	if len(agents) == 0 {
		k.touchHealth()
		return nil
	}
	aggregated := 0
	for _, id := range agents {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		tx, err := k.registry.AggregateEpoch(ctx, id)
		if err != nil {
			logger.Warn("keeper aggregate epoch failed",
				logger.String("agent_id", id), logger.Error(err))
			continue
		}
		aggregated++
		logger.Info("keeper aggregated epoch",
			logger.String("agent_id", id),
			logger.String("tx", tx))
	}
	if aggregated > 0 {
		logger.Info("keeper batch done", logger.Int("aggregated", aggregated))
	}
	k.touchHealth()
	return nil
}

// scanInactive fetches inactive agents and triggers decay for each.
func (k *Keeper) scanInactive(ctx context.Context) error {
	agents, err := k.registry.InactiveAgents(ctx, k.inactiveAge, k.pendingSize)
	if err != nil {
		return fmt.Errorf("fetch inactive agents: %w", err)
	}
	if len(agents) == 0 {
		k.touchHealth()
		return nil
	}
	decayed := 0
	for _, id := range agents {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		tx, err := k.registry.TickDecay(ctx, id)
		if err != nil {
			logger.Warn("keeper tick decay failed",
				logger.String("agent_id", id), logger.Error(err))
			continue
		}
		decayed++
		logger.Info("keeper decayed inactive agent",
			logger.String("agent_id", id),
			logger.String("tx", tx))
	}
	if decayed > 0 {
		logger.Info("keeper inactive scan done", logger.Int("decayed", decayed))
	}
	k.touchHealth()
	return nil
}

// touchHealth reports tick completion to the HealthTracker if configured.
func (k *Keeper) touchHealth() {
	if k.health != nil {
		k.health.TouchKeeper()
	}
}
