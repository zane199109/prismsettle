package demo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/errno"
)

// SessionRepo persists demo sessions and their message streams.
type SessionRepo struct {
	db *gorm.DB
}

func NewSessionRepo(db *gorm.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) CreateSession(ctx context.Context, s *model.DemoSession) error {
	s.CreatedAt = time.Now()
	return r.db.WithContext(ctx).Create(s).Error
}

// GetSession loads a session by id.
func (r *SessionRepo) GetSession(ctx context.Context, id string) (*model.DemoSession, error) {
	var sess model.DemoSession
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&sess).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errno.ErrNotFound
		}
		return nil, err
	}
	return &sess, nil
}

// GetByJobID loads the most recent session that created the given job.
func (r *SessionRepo) GetByJobID(ctx context.Context, jobID string) (*model.DemoSession, error) {
	var sess model.DemoSession
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobID).Order("created_at DESC").First(&sess).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &sess, nil
}

// UpdateSession patches mutable fields (state / job_id / result / finished_at).
func (r *SessionRepo) UpdateSession(ctx context.Context, s *model.DemoSession) error {
	return r.db.WithContext(ctx).Model(&model.DemoSession{}).
		Where("id = ?", s.ID).
		Updates(map[string]interface{}{
			"job_id":      s.JobID,
			"state":       s.State,
			"result":      s.Result,
			"finished_at": s.FinishedAt,
		}).Error
}

func (r *SessionRepo) ListMessages(ctx context.Context, sessionID string) ([]model.DemoMessage, error) {
	var msgs []model.DemoMessage
	err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("step ASC, id ASC").
		Find(&msgs).Error
	if err != nil {
		return nil, errno.ErrInternal
	}
	return msgs, nil
}

func (r *SessionRepo) AppendMessage(ctx context.Context, m *model.DemoMessage) error {
	m.CreatedAt = time.Now()
	return r.db.WithContext(ctx).Create(m).Error
}
