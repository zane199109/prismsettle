package repository

import (
	"context"
	"strings"

	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
	"gorm.io/gorm"
)

// GrabAttemptRepository persists agent grab attempts (success/failure with
// reason) so operators can audit why their agent lost a job competition.
type GrabAttemptRepository struct {
	db *gorm.DB
}

// NewGrabAttemptRepository creates a GrabAttemptRepository.
func NewGrabAttemptRepository(db *gorm.DB) *GrabAttemptRepository {
	return &GrabAttemptRepository{db: db}
}

// Create inserts a grab attempt record.
func (r *GrabAttemptRepository) Create(ctx context.Context, att *model.GrabAttempt) error {
	if err := r.db.WithContext(ctx).Create(att).Error; err != nil {
		logger.Errorf("create grab attempt failed",
			logger.String("agent", att.AgentID),
			logger.String("job", att.JobID),
			logger.Error(err))
		return errno.ErrInternal
	}
	return nil
}

// List returns paginated grab attempts, optionally filtered by agent/job.
func (r *GrabAttemptRepository) List(
	ctx context.Context,
	chainName, agentID, jobID string,
	page, size int,
) ([]model.GrabAttempt, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	q := r.db.WithContext(ctx).Model(&model.GrabAttempt{})
	if chainName != "" {
		q = q.Where("chain_name = ?", chainName)
	}
	if agentID != "" {
		q = q.Where("LOWER(agent_id) = ?", strings.ToLower(agentID))
	}
	if jobID != "" {
		q = q.Where("LOWER(job_id) = ?", strings.ToLower(jobID))
	}
	var (
		list  []model.GrabAttempt
		total int64
	)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errno.ErrInternal
	}
	if err := q.Order("id DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&list).Error; err != nil {
		return nil, 0, errno.ErrInternal
	}
	return list, total, nil
}
