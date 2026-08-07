package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
	"gorm.io/gorm"
)

// prismEventTypes is the set of event types emitted by PrismSettleRegistry.
var prismEventTypes = []string{
	string(model.TypePrismAgentRegistered),
	string(model.TypePrismValidationSubmitted),
	string(model.TypePrismAggregated),
	string(model.TypePrismStaked),
	string(model.TypePrismSlashed),
}

// GetPrismEvents returns paginated PrismSettle registry events for an agent.
// agentId is matched against the `To` column (where the parser stores the
// agentId hex). When agentId is empty, all PrismSettle events are returned.
func (r *ChainEventRepository) GetPrismEvents(
	ctx context.Context,
	chainName, contractAddr, agentID string,
	page, size int,
) ([]model.ChainEvent, int64, error) {
	var (
		list  []model.ChainEvent
		total int64
	)

	query := r.db.WithContext(ctx).
		Model(&model.ChainEvent{}).
		Where("chain_name = ?", chainName)

	if contractAddr != "" {
		query = query.Where("LOWER(contract) = ?", strings.ToLower(contractAddr))
	}

	if agentID != "" {
		// agentId is stored as a 0x-prefixed 32-byte hex in `To`. Normalize
		// the input the same way so the equality match is stable regardless
		// of how the client formatted the agentId.
		query = query.Where("LOWER(\"to\") = ?", strings.ToLower(agentID))
	}

	query = query.Where("event_type IN (?)", prismEventTypes)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("repo count prism events failed", logger.Error(err))
		return nil, 0, errno.ErrInternal
	}

	offset := (page - 1) * size
	err := query.
		Order("block_number DESC, log_index DESC").
		Offset(offset).
		Limit(size).
		Find(&list).Error
	if err != nil {
		logger.Errorf("repo get prism events failed", logger.Error(err))
		return nil, 0, errno.ErrInternal
	}

	return list, total, nil
}

// GetPrismScore returns the latest Aggregated event for an agent, which
// carries the most recent aggregatedScore. Returns ErrNotFound if no
// aggregation has been emitted yet.
func (r *ChainEventRepository) GetPrismScore(
	ctx context.Context,
	chainName, contractAddr, agentID string,
) (*model.ChainEvent, error) {
	var ev model.ChainEvent

	query := r.db.WithContext(ctx).
		Model(&model.ChainEvent{}).
		Where("chain_name = ?", chainName).
		Where("event_type = ?", model.TypePrismAggregated).
		Where("LOWER(\"to\") = ?", strings.ToLower(agentID))

	if contractAddr != "" {
		query = query.Where("LOWER(contract) = ?", strings.ToLower(contractAddr))
	}

	err := query.
		Order("block_number DESC, log_index DESC").
		First(&ev).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errno.ErrNotFound
		}
		logger.Errorf("repo get prism score failed", logger.Error(err))
		return nil, errno.ErrInternal
	}
	return &ev, nil
}

// CountPrismValidations returns the number of ValidationSubmitted events
// for an agent. Used by the demo dashboard to surface "validations received".
func (r *ChainEventRepository) CountPrismValidations(
	ctx context.Context,
	chainName, contractAddr, agentID string,
) (int64, error) {
	var total int64

	query := r.db.WithContext(ctx).
		Model(&model.ChainEvent{}).
		Where("chain_name = ?", chainName).
		Where("event_type = ?", model.TypePrismValidationSubmitted).
		Where("LOWER(\"to\") = ?", strings.ToLower(agentID))

	if contractAddr != "" {
		query = query.Where("LOWER(contract) = ?", strings.ToLower(contractAddr))
	}

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("repo count prism validations failed", logger.Error(err))
		return 0, errno.ErrInternal
	}
	return total, nil
}

// prismJobEventTypes is the set of PrismSettleJob lifecycle event types.
var prismJobEventTypes = []string{
	string(model.TypePrismJobCreated),
	string(model.TypePrismJobFunded),
	string(model.TypePrismJobAssigned),
	string(model.TypePrismJobSubmitted),
	string(model.TypePrismJobCompleted),
	string(model.TypePrismJobRefunded),
	string(model.TypePrismDisputed),
	string(model.TypePrismDisputeResolved),
}

// ListPrismJobs returns paginated PrismSettleJob lifecycle events, optionally
// filtered by job state. Phase 7 task 7.1 / FR-A08.
//
// state filter mapping (FR-A10):
//   - "open"      → JobCreated
//   - "funded"    → JobFunded
//   - "submitted" → JobSubmitted
//   - "completed" → JobCompleted
//   - "refunded"  → JobRefunded
//   - "disputed"  → Disputed
//   - "resolved"  → DisputeResolved
//   - ""           → all job events
func (r *ChainEventRepository) ListPrismJobs(
	ctx context.Context,
	chainName, contractAddr, state string,
	page, size int,
) ([]model.ChainEvent, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	var (
		list  []model.ChainEvent
		total int64
	)
	q := r.db.WithContext(ctx).
		Model(&model.ChainEvent{}).
		Where("chain_name = ?", chainName).
		Where("event_type IN ?", prismJobEventTypes)
	if contractAddr != "" {
		q = q.Where("LOWER(contract) = ?", strings.ToLower(contractAddr))
	}
	if et := jobStateToEventType(state); et != "" {
		q = q.Where("event_type = ?", et)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errno.ErrInternal
	}
	if err := q.Order("block_number DESC, log_index DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&list).Error; err != nil {
		return nil, 0, errno.ErrInternal
	}
	return list, total, nil
}

// jobStateToEventType maps a state string to the corresponding event type.
// Returns "" when state is unrecognized (no filter applied).
func jobStateToEventType(state string) model.EventType {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "open":
		return model.TypePrismJobCreated
	case "funded":
		return model.TypePrismJobFunded
	case "submitted":
		return model.TypePrismJobSubmitted
	case "completed":
		return model.TypePrismJobCompleted
	case "refunded":
		return model.TypePrismJobRefunded
	case "disputed":
		return model.TypePrismDisputed
	case "resolved":
		return model.TypePrismDisputeResolved
	}
	return ""
}

// GetPrismJobTimeline returns all events for a specific jobId, ordered by
// block_number ASC, log_index ASC (chronological). Phase 7 task 7.1 /
// FR-JM02. The jobId is matched against the `To` column where the
// PrismSettleJob parser stores the jobId hex.
func (r *ChainEventRepository) GetPrismJobTimeline(
	ctx context.Context,
	chainName, jobID string,
) ([]model.ChainEvent, error) {
	var list []model.ChainEvent
	err := r.db.WithContext(ctx).
		Model(&model.ChainEvent{}).
		Where("chain_name = ?", chainName).
		Where("event_type IN ?", prismJobEventTypes).
		Where("LOWER(\"to\") = ?", strings.ToLower(jobID)).
		Order("block_number ASC, log_index ASC").
		Find(&list).Error
	if err != nil {
		return nil, errno.ErrInternal
	}
	return list, nil
}

// GetPrismReputationHistory returns the most recent N aggregation/validation
// events for an agent, used by GET /reputation/history (FR-A12). Default N=30.
func (r *ChainEventRepository) GetPrismReputationHistory(
	ctx context.Context,
	chainName, contractAddr, agentID string,
	limit int,
) ([]model.ChainEvent, error) {
	if limit < 1 || limit > 200 {
		limit = 30
	}
	var list []model.ChainEvent
	q := r.db.WithContext(ctx).
		Model(&model.ChainEvent{}).
		Where("chain_name = ?", chainName).
		Where("event_type IN ?", []string{
			string(model.TypePrismAggregated),
			string(model.TypePrismValidationSubmitted),
			string(model.TypePrismSlashed),
		}).
		Where("LOWER(\"to\") = ?", strings.ToLower(agentID))
	if contractAddr != "" {
		q = q.Where("LOWER(contract) = ?", strings.ToLower(contractAddr))
	}
	err := q.Order("block_number DESC, log_index DESC").
		Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, errno.ErrInternal
	}
	return list, nil
}

// GetPrismShardActivity returns per-shard validation counts for the last
// `sinceBlocks` blocks. Phase 7 task 7.1 / FR-A04. Implementation is a simple
// GROUP BY on the validation events; the shard ID is stored in the `Symbol`
// field by the parser (shard is topic[2] but currently not extracted — this
// returns counts keyed by event_type until shard extraction is added in a
// later phase).
//
// NOTE: This returns a placeholder structure; full shard-level breakdown
// requires extending the parser to extract shard from topics[2]. Tracked for
// Phase 9.
func (r *ChainEventRepository) GetPrismShardActivity(
	ctx context.Context,
	chainName, contractAddr string,
) ([]model.ShardActivityVO, error) {
	// Aggregate validation counts grouped by the shard value currently stored
	// in the Symbol column. When Symbol is empty (parser hasn't extracted
	// shard), all such rows collapse into shard 0.
	//
	// Subquery + outer GROUP BY: the CASE alias becomes a real column inside
	// the derived table, so the outer `GROUP BY "shard_id"` resolves (quoting
	// a CASE alias directly would look for an actual column and fail).
	var rows []model.ShardActivityVO
	sub := r.db.WithContext(ctx).
		Model(&model.ChainEvent{}).
		Select(`
			CASE
				WHEN COALESCE(symbol, '') = '' THEN 0
				WHEN symbol ~ '^[0-9]+$' THEN CAST(symbol AS SMALLINT)
				ELSE 0
			END AS shard_id,
			COUNT(*)::BIGINT AS validations,
			MAX(block_time)::BIGINT AS last_activity
		`).
		Where("chain_name = ?", chainName).
		Where("event_type = ?", model.TypePrismValidationSubmitted)
	if contractAddr != "" {
		sub = sub.Where("LOWER(contract) = ?", strings.ToLower(contractAddr))
	}
	// The subquery is an aggregate (COUNT/MAX) so the raw symbol column must
	// be grouped; the CASE alias then becomes a real column in the derived
	// table and the outer GROUP BY "shard_id" resolves.
	sub = sub.Group("symbol")
	if err := r.db.WithContext(ctx).
		Table("(?) AS t", sub).
		Select("shard_id, SUM(validations) AS validations, MAX(last_activity) AS last_activity").
		Group("shard_id").
		Scan(&rows).Error; err != nil {
		return nil, errno.ErrInternal
	}
	return rows, nil
}
