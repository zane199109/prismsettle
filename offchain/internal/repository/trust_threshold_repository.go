package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/errno"
	"gorm.io/gorm"
)

// TrustThresholdRepository provides access to the trust_thresholds table
// (Phase 7 task 7.2). agent_id = "" represents the global default; per-agent
// rows override it.
type TrustThresholdRepository struct {
	db *gorm.DB
}

// NewTrustThresholdRepository creates a new TrustThresholdRepository.
func NewTrustThresholdRepository(db *gorm.DB) *TrustThresholdRepository {
	return &TrustThresholdRepository{db: db}
}

// Get returns the threshold row for an agent, falling back to the global
// default (agent_id = "") when no per-agent row exists.
func (r *TrustThresholdRepository) Get(ctx context.Context, agentID string) (*model.TrustThreshold, error) {
	agentID = strings.ToLower(strings.TrimSpace(agentID))
	var rec model.TrustThreshold
	// Try per-agent row first.
	err := r.db.WithContext(ctx).
		Where("LOWER(agent_id) = ?", agentID).
		First(&rec).Error
	if err == nil {
		return &rec, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errno.ErrInternal
	}
	// Fall back to global default.
	err = r.db.WithContext(ctx).
		Where("agent_id = ?", "").
		First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// No default row yet — return in-memory default values.
			return &model.TrustThreshold{
				AgentID:        "",
				AllowThreshold: "800000000000000000", // 0.8e18
				DenyThreshold:  "300000000000000000", // 0.3e18
			}, nil
		}
		return nil, errno.ErrInternal
	}
	return &rec, nil
}

// Upsert inserts or updates a threshold row. agent_id = "" updates the global
// default. Used by the POST /trust/thresholds admin endpoint.
func (r *TrustThresholdRepository) Upsert(
	ctx context.Context,
	agentID, allowThreshold, denyThreshold string,
) error {
	agentID = strings.ToLower(strings.TrimSpace(agentID))
	rec := model.TrustThreshold{
		AgentID:        agentID,
		AllowThreshold: allowThreshold,
		DenyThreshold:  denyThreshold,
	}
	// Use agent_id as the conflict target (unique index idx_trust_agent).
	if err := r.db.WithContext(ctx).Save(&rec).Error; err != nil {
		return errno.ErrInternal
	}
	return nil
}
