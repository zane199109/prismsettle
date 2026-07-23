// Package repository — perf_result_repository.go
//
// PerfResultRepository stores V0 vs V1 benchmark results (Phase 9 task 9.6).
// The /perf/v0-v1-comparison endpoint reads the latest row to serve real
// data to the dashboard, replacing the Phase 7 mock.
//
// The Go load generator (offchain/cmd/bench) is the writer; the REST API
// is the reader. Manual CLI insertion is also supported for cases where
// the load test runs outside the binary (e.g. cast-based scripts).

package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
)

// PerfResultRepository handles PerfResult persistence.
type PerfResultRepository struct {
	db *gorm.DB
}

// NewPerfResultRepository constructs a new repository bound to the given GORM DB.
func NewPerfResultRepository(db *gorm.DB) *PerfResultRepository {
	return &PerfResultRepository{db: db}
}

// Insert persists a single benchmark run. Returns ErrInternal on DB failure.
// RunID must be unique — re-inserting the same RunID fails with a unique
// constraint violation (callers should generate a fresh ID per run).
func (r *PerfResultRepository) Insert(ctx context.Context, rec *model.PerfResult) error {
	if rec == nil {
		return errno.ErrInvalidParam
	}
	if err := r.db.WithContext(ctx).Create(rec).Error; err != nil {
		logger.Errorf("insert perf_result failed",
			logger.String("run_id", rec.RunID),
			logger.Error(err))
		return errno.ErrInternal
	}
	return nil
}

// GetLatest returns the most recent PerfResult row, or nil with no error
// when the table is empty (the service falls back to mock values in that
// case, preserving Phase 7 behavior until the first real run lands).
func (r *PerfResultRepository) GetLatest(ctx context.Context) (*model.PerfResult, error) {
	var rec model.PerfResult
	err := r.db.WithContext(ctx).
		Order("created_at DESC, id DESC").
		First(&rec).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		logger.Errorf("get latest perf_result failed", logger.Error(err))
		return nil, errno.ErrInternal
	}
	return &rec, nil
}

// ListSince returns all runs created at or after the given timestamp,
// oldest-first. Useful for trend charts on the /perf page.
func (r *PerfResultRepository) ListSince(ctx context.Context, since time.Time, limit int) ([]model.PerfResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := r.db.WithContext(ctx).
		Order("created_at ASC, id ASC").
		Limit(limit)
	if !since.IsZero() {
		q = q.Where("created_at >= ?", since)
	}
	var recs []model.PerfResult
	if err := q.Find(&recs).Error; err != nil {
		logger.Errorf("list perf_results failed", logger.Error(err))
		return nil, errno.ErrInternal
	}
	return recs, nil
}
