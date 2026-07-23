package evaluator

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// DecisionLog records every decision the Evaluator makes: complete, reject,
// arbitration, fallback. Reorg-affected rows are flagged invalid rather than
// deleted so the audit trail is preserved (project convention).
//
// One row per (jobId, source) decision. The (job_id, source) unique index
// enforces idempotency: the same job can have at most one main-path decision
// (source=1) and one arbitration decision (source=2).
type DecisionLog struct {
	ID         uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	JobID      string         `gorm:"column:job_id;type:varchar(128;index:idx_job_source,unique" json:"job_id"`
	Source     uint8          `gorm:"column:source;index:idx_job_source,unique" json:"source"` // 1=main, 2=arbitration
	Decision   string         `gorm:"column:decision;type:varchar(32)" json:"decision"`        // complete / reject / dispute_resolved / fallback
	Score      string         `gorm:"column:score;type:varchar(64)" json:"score"`              // decimal string, e.g. "600000000000000000"
	Reason     string         `gorm:"column:reason;type:varchar(512)" json:"reason"`
	TxHash     string         `gorm:"column:tx_hash;type:varchar(128" json:"tx_hash"`
	CallerRole string         `gorm:"column:caller_role;type:varchar(64)" json:"caller_role"` // SD §4.5.7 audit field
	Invalid    bool           `gorm:"column:invalid;index" json:"invalid"`                    // true if reorg-affected
	CreatedAt  time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default GORM table name.
func (DecisionLog) TableName() string { return "decision_logs" }

// DecisionStore is the persistence interface for decision logs. The Evaluator
// uses it to record decisions and to check idempotency (has this job already
// been decided?).
type DecisionStore interface {
	// Insert persists a new decision. Returns an error if a row with the same
	// (jobId, source) already exists (idempotency guard).
	Insert(ctx context.Context, d *DecisionLog) error
	// HasDecision reports whether a decision exists for (jobId, source).
	HasDecision(ctx context.Context, jobId string, source uint8) (bool, error)
	// MarkInvalid flags the latest decision for (jobId, source) as invalid
	// (reorg rollback). Returns ErrNotFound if no row exists.
	MarkInvalid(ctx context.Context, jobId string, source uint8) error
	// Recent returns the most recent N decisions for audit / display.
	Recent(ctx context.Context, limit int) ([]DecisionLog, error)
}

// --- GORM implementation ------------------------------------------------

// GormDecisionStore is the default DecisionStore backed by GORM.
type GormDecisionStore struct {
	db *gorm.DB
}

// NewGormDecisionStore builds a DecisionStore on the given *gorm.DB. The
// caller is responsible for AutoMigrate(DecisionLog{}) once at boot.
func NewGormDecisionStore(db *gorm.DB) *GormDecisionStore {
	return &GormDecisionStore{db: db}
}

// Insert persists a new decision. The unique (job_id, source) index makes
// concurrent inserts safe: the second insert fails with a duplicate-key error,
// which the Evaluator treats as "already decided" (idempotency).
func (s *GormDecisionStore) Insert(ctx context.Context, d *DecisionLog) error {
	return s.db.WithContext(ctx).Create(d).Error
}

// HasDecision reports whether any decision exists for (jobId, source).
func (s *GormDecisionStore) HasDecision(ctx context.Context, jobId string, source uint8) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&DecisionLog{}).
		Where("job_id = ? AND source = ? AND invalid = false", jobId, source).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// MarkInvalid flags the latest decision for (jobId, source) as invalid.
func (s *GormDecisionStore) MarkInvalid(ctx context.Context, jobId string, source uint8) error {
	res := s.db.WithContext(ctx).Model(&DecisionLog{}).
		Where("job_id = ? AND source = ? AND invalid = false", jobId, source).
		Update("invalid", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Recent returns the most recent N decisions ordered by created_at desc.
func (s *GormDecisionStore) Recent(ctx context.Context, limit int) ([]DecisionLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var out []DecisionLog
	err := s.db.WithContext(ctx).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&out).Error
	return out, err
}
