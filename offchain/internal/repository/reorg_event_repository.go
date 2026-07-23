// Package repository — reorg_event_repository.go
//
// ReorgEventRepository persists ReorgEvent rows produced by the listener's
// reorg-detection path (Phase 9 task 9.4). The /perf/reorg-feed endpoint
// reads from this table to surface real reorgs on the dashboard.
//
// Design notes:
//   - Writes are append-only — reorgs are never updated or deleted.
//   - Reads are time-ordered (latest first) and bounded by a limit param
//     to keep the feed payload small.
//   - The repository does NOT bump any in-memory counter; the listener
//     is responsible for calling HealthTracker.IncReorg separately so
//     the /health endpoint's reorg_count stays decoupled from the DB.

package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
)

// ReorgEventRepository handles ReorgEvent persistence.
type ReorgEventRepository struct {
	db *gorm.DB
}

// NewReorgEventRepository constructs a new repository bound to the given GORM DB.
func NewReorgEventRepository(db *gorm.DB) *ReorgEventRepository {
	return &ReorgEventRepository{db: db}
}

// Insert persists a single reorg event. Called by the listener right
// after RollbackEvents completes — if this insert fails, the reorg is
// still rolled back in chain_events, but the dashboard feed will miss it.
// We log + swallow the error to avoid crashing the listener's main loop.
func (r *ReorgEventRepository) Insert(
	ctx context.Context,
	chainName, contractAddr string,
	fromBlock, toBlock uint64,
	oldHash, newHash string,
	rollbackDepth int,
	rolledBackRows int64,
) error {
	rec := model.ReorgEvent{
		ChainName:      chainName,
		ContractAddr:   contractAddr,
		FromBlock:      fromBlock,
		ToBlock:        toBlock,
		OldHash:        oldHash,
		NewHash:        newHash,
		RollbackDepth:  rollbackDepth,
		RolledBackRows: rolledBackRows,
		DetectedAt:     time.Now(),
	}
	if err := r.db.WithContext(ctx).Create(&rec).Error; err != nil {
		logger.Errorf("insert reorg_event failed",
			logger.String("chain", chainName),
			logger.Uint64("from", fromBlock),
			logger.Uint64("to", toBlock),
			logger.Error(err))
		return errno.ErrInternal
	}
	return nil
}

// ListRecent returns the most recent N reorg events across all chains
// (or filtered by chainName when non-empty), ordered newest-first.
// Used by GET /perf/reorg-feed (Phase 9 task 9.5).
func (r *ReorgEventRepository) ListRecent(
	ctx context.Context,
	chainName string,
	limit int,
) ([]model.ReorgEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := r.db.WithContext(ctx).Order("detected_at DESC, id DESC").Limit(limit)
	if chainName != "" {
		q = q.Where("chain_name = ?", chainName)
	}
	var recs []model.ReorgEvent
	if err := q.Find(&recs).Error; err != nil {
		logger.Errorf("list reorg_events failed", logger.String("chain", chainName), logger.Error(err))
		return nil, errno.ErrInternal
	}
	return recs, nil
}

// CountSince returns the total number of reorg events since the given
// timestamp. Used by /health to compute reorg_count when the in-memory
// counter has reset (e.g. after a restart). Pass time.Time{} for all-time.
func (r *ReorgEventRepository) CountSince(ctx context.Context, since time.Time) (int64, error) {
	q := r.db.WithContext(ctx).Model(&model.ReorgEvent{})
	if !since.IsZero() {
		q = q.Where("detected_at >= ?", since)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, errno.ErrInternal
	}
	return n, nil
}
