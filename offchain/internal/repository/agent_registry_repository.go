package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentRegistryRepository provides read/write access to the agent_registry
// mirror table. Writes happen via UpsertFromEvent when the indexer observes
// an AgentRegistered event; reads serve the /agents endpoints.
type AgentRegistryRepository struct {
	db *gorm.DB
}

// NewAgentRegistryRepository creates a new AgentRegistryRepository.
func NewAgentRegistryRepository(db *gorm.DB) *AgentRegistryRepository {
	return &AgentRegistryRepository{db: db}
}

// UpsertFromEvent inserts or updates an agent_registry row from an
// AgentRegistered ChainEvent. The unique key is (chain_name, agent_id).
// metadata/endpoint are left empty here — the indexer parser does not
// currently decode the metadata string (no free-form text field on
// ChainEvent). Phase 9 may backfill metadata via raw-log replay.
func (r *AgentRegistryRepository) UpsertFromEvent(ctx context.Context, ev *model.ChainEvent) error {
	if ev == nil {
		return errno.ErrInvalidParam
	}
	rec := model.AgentRegistryRecord{
		ChainName:    ev.ChainName,
		AgentID:      strings.ToLower(ev.To),
		Owner:        strings.ToLower(ev.From),
		RegisteredAt: ev.BlockTime,
		TxHash:       ev.TxHash,
		BlockNumber:  ev.BlockNumber,
	}
	// On conflict, update owner + latest block info (re-registration is allowed).
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "chain_name"}, {Name: "agent_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"owner":         rec.Owner,
			"registered_at": rec.RegisteredAt,
			"tx_hash":       rec.TxHash,
			"block_number":  rec.BlockNumber,
		}),
	}).Create(&rec).Error; err != nil {
		logger.Errorf("upsert agent_registry failed",
			logger.String("agent", rec.AgentID),
			logger.Error(err))
		return errno.ErrInternal
	}
	return nil
}

// List returns paginated agent_registry rows for a chain.
func (r *AgentRegistryRepository) List(
	ctx context.Context,
	chainName string,
	page, size int,
) ([]model.AgentRegistryRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	var (
		list  []model.AgentRegistryRecord
		total int64
	)
	q := r.db.WithContext(ctx).Model(&model.AgentRegistryRecord{}).
		Where("chain_name = ?", chainName)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errno.ErrInternal
	}
	if err := q.Order("registered_at DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&list).Error; err != nil {
		return nil, 0, errno.ErrInternal
	}
	return list, total, nil
}

// Get returns a single agent by chain + agentId.
func (r *AgentRegistryRepository) Get(
	ctx context.Context,
	chainName, agentID string,
) (*model.AgentRegistryRecord, error) {
	var rec model.AgentRegistryRecord
	err := r.db.WithContext(ctx).
		Where("chain_name = ?", chainName).
		Where("LOWER(agent_id) = ?", strings.ToLower(agentID)).
		First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errno.ErrNotFound
		}
		return nil, errno.ErrInternal
	}
	return &rec, nil
}
