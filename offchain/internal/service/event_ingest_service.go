package service

import (
	"context"

	"github.com/zane/web3-offchain/internal/repository"
	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/logger"
)

// EventIngestService handles the persistence of blockchain events
// It provides methods for batch saving events and rolling back during chain reorgs
type EventIngestService struct {
	ceRepo    *repository.ChainEventRepository
	agentRepo *repository.AgentRegistryRepository // optional, nil-safe
}

// NewEventIngestService creates a new EventIngestService instance
func NewEventIngestService(ceRepo *repository.ChainEventRepository) *EventIngestService {
	return &EventIngestService{ceRepo: ceRepo}
}

// NewEventIngestServiceWithAgentRepo creates an EventIngestService wired with
// the agent_registry mirror repository. Phase 7 task 7.1: when an
// AgentRegistered event is persisted, the agent_registry table is upserted
// so /agents endpoint queries don't need to scan raw events.
func NewEventIngestServiceWithAgentRepo(
	ceRepo *repository.ChainEventRepository,
	agentRepo *repository.AgentRegistryRepository,
) *EventIngestService {
	return &EventIngestService{ceRepo: ceRepo, agentRepo: agentRepo}
}

// BatchSaveChainEvents is the unified entry point for saving parsed blockchain events
// It delegates to the repository for batch insertion with idempotency
func (es *EventIngestService) BatchSaveChainEvents(ctx context.Context, events []*model.ChainEvent) error {
	if len(events) == 0 {
		return nil
	}

	err := es.ceRepo.BatchSaveChainEvents(ctx, events)
	if err != nil {
		logger.Errorf("batch save chain events failed",
			logger.Int("count", len(events)),
			logger.Error(err),
		)
		// Alerting can be added here (e.g., Telegram, PagerDuty)
		return err
	}

	// Phase 7 task 7.1: mirror AgentRegistered events into agent_registry so
	// the /agents endpoint can query metadata without re-parsing events. Best
	// effort — failures here do NOT fail the ingest path (events are already
	// safely stored in chain_events).
	if es.agentRepo != nil {
		for i := range events {
			if events[i].EventType != model.TypePrismAgentRegistered {
				continue
			}
			if err := es.agentRepo.UpsertFromEvent(ctx, events[i]); err != nil {
				logger.Warn("agent_registry mirror upsert failed (non-fatal)",
					logger.String("agent", events[i].To),
					logger.Error(err))
			}
		}
	}

	return nil
}

// RollbackEvents deletes events within a block range to handle chain reorganization (reorg)
// and returns the number of rows deleted. This ensures data consistency when old blocks
// are discarded. The row count is used by the reorg audit log (Phase 9 P1-F).
func (es *EventIngestService) RollbackEvents(
	ctx context.Context,
	chainName string,
	contractAddr string,
	startRollbackBlock uint64,
	dbLastBlock uint64,
) (int64, error) {
	// No need to rollback if the starting block is ahead of the last synced block
	if startRollbackBlock > dbLastBlock {
		return 0, nil
	}

	rows, err := es.ceRepo.RollbackEvents(ctx, chainName, contractAddr, startRollbackBlock, dbLastBlock)
	if err != nil {
		logger.Errorf("rollback chain events failed",
			logger.String("chain", chainName),
			logger.String("contract", contractAddr),
			logger.Uint64("from_block", startRollbackBlock),
			logger.Uint64("to_block", dbLastBlock),
			logger.Error(err),
		)
		// Alerting can be added here
		return 0, err
	}

	return rows, nil
}
