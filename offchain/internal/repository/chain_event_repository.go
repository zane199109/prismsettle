package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ChainEventRepository handles data operations for on-chain contract events
// Provides batch saving, rollback, and token info query with production optimizations
type ChainEventRepository struct {
	db *gorm.DB
}

// NewChainEventRepository creates a new ChainEventRepository instance
// Accepts a gorm DB instance via dependency injection
func NewChainEventRepository(db *gorm.DB) *ChainEventRepository {
	return &ChainEventRepository{db: db}
}

const (
	// batchSaveSize defines the batch size for bulk insert to optimize DB performance
	batchSaveSize = 200

	// slowBatchSaveThreshold logs a warning if batch save exceeds this duration
	slowBatchSaveThreshold = 1 * time.Second
)

// BatchSaveChainEvents saves a list of on-chain events in batches
// Ensures idempotency using ON CONFLICT DO NOTHING (tx_hash + log_index unique constraint)
// Skips duplicate events and supports large volume insertion with performance tuning
func (r *ChainEventRepository) BatchSaveChainEvents(
	ctx context.Context,
	chainEvents []*model.ChainEvent,
) error {
	// Fast return for empty input
	if len(chainEvents) == 0 {
		return nil
	}

	// Check if context is already canceled to avoid unnecessary DB operation
	if err := ctx.Err(); err != nil {
		logger.Warn("batch save skipped: context canceled", logger.Error(err))
		return err
	}

	start := time.Now()

	// Batch insert with conflict detection (duplicate events are ignored)
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			// Unique key for blockchain events: transaction hash + log index .
			// You have to make sure that there is a unique index on (tx_hash, log_index)
			// in the database for this to work correctly
			Columns: []clause.Column{
				{Name: "tx_hash"},
				{Name: "log_index"},
			},
			DoNothing: true, // Do not insert on duplicate key conflict
		}).
		CreateInBatches(chainEvents, batchSaveSize).Error

	cost := time.Since(start)

	// Monitor slow queries for production performance tuning
	if cost > slowBatchSaveThreshold {
		logger.Warn("slow batch save chain events",
			logger.Int("count", len(chainEvents)),
			logger.Duration("cost", cost),
		)
	} else {
		logger.Debug("batch save chain events success",
			logger.Int("count", len(chainEvents)),
			logger.Duration("cost", cost),
		)
	}

	if err != nil {
		logger.Errorf("batch save chain events failed",
			logger.Int("count", len(chainEvents)),
			logger.Error(err),
		)
		return err
	}

	return nil
}

// GetTokenInfoFromEvent retrieves token symbol and decimals from existing events
// Used when token metadata is not available from contract directly
func (r *ChainEventRepository) GetTokenInfoFromEvent(
	ctx context.Context,
	chainName string,
	tokenAddr string,
) (*model.TokenInfo, error) {
	var event model.ChainEvent

	// LOWER(...) on both sides because token_addr may have been written with
	// EIP-55 checksum casing by an older parser version, while callers pass
	// the canonical lowercase form. Inconsistent casing causes silent misses.
	err := r.db.WithContext(ctx).
		Where("chain_name = ?", chainName).
		Where("LOWER(token_addr) = ?", strings.ToLower(tokenAddr)).
		Select("symbol", "decimals").
		First(&event).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errno.ErrNotFound
		}
		logger.Errorf("failed to query token info from events",
			logger.String("chain", chainName),
			logger.String("token", tokenAddr),
			logger.Error(err),
		)
		return nil, err
	}

	return &model.TokenInfo{
		Symbol:   event.Symbol,
		Decimals: event.Decimals,
	}, nil
}

// RollbackEvents deletes events within a block range during chain reorg
// and returns the number of rows actually deleted. This is critical for
// data consistency when blocks are orphaned. The row count is used by the
// reorg event audit log (Phase 9 P1-F) so /perf/reorg-feed can show how
// many events were affected per reorg, not just the block range.
func (r *ChainEventRepository) RollbackEvents(
	ctx context.Context,
	chainName string,
	contractAddr string,
	fromBlock uint64,
	toBlock uint64,
) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("chain_name = ? AND contract = ? AND block_number >= ? AND block_number <= ?",
			chainName, contractAddr, fromBlock, toBlock).
		Delete(&model.ChainEvent{})
	return res.RowsAffected, res.Error
}
