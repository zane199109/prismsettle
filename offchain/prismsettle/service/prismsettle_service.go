package service

import (
	"context"
	"math/big"
	"sort"
	"strings"
	"sync"

	"github.com/zane/web3-offchain/internal/repository"
	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
)

// PrismSettleService exposes read-side queries over PrismSettleRegistry
// events stored by the indexer. It is intentionally thin: heavy lifting
// (filtering, pagination) lives in the repository; this layer normalizes
// inputs and translates errors into domain errno values.
//
// Phase 7 task 7.1: extended with agent_registry + trust_threshold +
// jobs + timeline + reputation history + shard activity + perf endpoints.
//
// Phase 9 task 9.5/9.6: reorgRepo + perfRepo wired so /perf/reorg-feed and
// /perf/v0-v1-comparison return real data instead of Phase 7 mocks.
type PrismSettleService struct {
	ceRepo    *repository.ChainEventRepository
	agentRepo *repository.AgentRegistryRepository
	trustRepo *repository.TrustThresholdRepository
	reorgRepo *repository.ReorgEventRepository
	perfRepo  *repository.PerfResultRepository
	grabRepo  *repository.GrabAttemptRepository
	// scoreFetcher resolves live on-chain scores for agents with no
	// aggregation event in the DB (e.g. seed agents). Optional; nil-safe.
	scoreFetcher *ScoreFetcher
}

// NewPrismSettleService creates a PrismSettleService with the legacy
// chain-event repository only (back-compat for existing callers).
func NewPrismSettleService(ceRepo *repository.ChainEventRepository) *PrismSettleService {
	return &PrismSettleService{ceRepo: ceRepo}
}

// SetScoreFetcher wires the optional live-score resolver (main.go).
func (s *PrismSettleService) SetScoreFetcher(f *ScoreFetcher) {
	s.scoreFetcher = f
}

// RecordGrabAttempt persists an agent's grab attempt (success/failure+reason).
func (s *PrismSettleService) RecordGrabAttempt(ctx context.Context, att *model.GrabAttempt) error {
	if s.grabRepo == nil {
		return errno.ErrInternal
	}
	if att.AgentID == "" || att.JobID == "" {
		return errno.ErrInvalidParam
	}
	return s.grabRepo.Create(ctx, att)
}

// ListGrabAttempts returns paginated grab attempts, optionally filtered.
func (s *PrismSettleService) ListGrabAttempts(
	ctx context.Context,
	chainName, agentID, jobID string,
	page, size int,
) ([]model.GrabAttempt, int64, error) {
	if s.grabRepo == nil {
		return nil, 0, errno.ErrInternal
	}
	return s.grabRepo.List(ctx, chainName, agentID, jobID, page, size)
}

// NewPrismSettleServiceWithRepos creates a fully-wired PrismSettleService.
// Phase 7 main.go uses this constructor.
func NewPrismSettleServiceWithRepos(
	ceRepo *repository.ChainEventRepository,
	agentRepo *repository.AgentRegistryRepository,
	trustRepo *repository.TrustThresholdRepository,
) *PrismSettleService {
	return &PrismSettleService{
		ceRepo:    ceRepo,
		agentRepo: agentRepo,
		trustRepo: trustRepo,
	}
}

// NewPrismSettleServiceWithPerfRepos creates a service wired with the
// Phase 9 reorg + perf repositories so /perf/* endpoints serve real data.
// main.go (Phase 9) uses this constructor.
func NewPrismSettleServiceWithPerfRepos(
	ceRepo *repository.ChainEventRepository,
	agentRepo *repository.AgentRegistryRepository,
	trustRepo *repository.TrustThresholdRepository,
	reorgRepo *repository.ReorgEventRepository,
	perfRepo *repository.PerfResultRepository,
	grabRepo *repository.GrabAttemptRepository,
) *PrismSettleService {
	return &PrismSettleService{
		ceRepo:    ceRepo,
		agentRepo: agentRepo,
		trustRepo: trustRepo,
		reorgRepo: reorgRepo,
		perfRepo:  perfRepo,
		grabRepo:  grabRepo,
	}
}

// GetEvents returns paginated PrismSettle registry events for an agent.
// When agentID is empty, all registry events on the chain are returned.
// eventType optionally filters to a single event type (e.g. PRISM_JOB_CREATED).
// minID > 0 switches to incremental cursor mode (id > minID, ascending).
func (s *PrismSettleService) GetEvents(
	ctx context.Context, chainName, contractAddr, agentID, eventType string,
	minID uint64,
	page, size int,
) ([]model.ChainEvent, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	contractAddr = strings.TrimSpace(contractAddr)
	agentID = strings.TrimSpace(agentID)
	eventType = strings.TrimSpace(eventType)

	events, total, err := s.ceRepo.GetPrismEvents(
		ctx, chainName, contractAddr, agentID, eventType, minID, page, size,
	)
	if err != nil {
		logger.Warn("prismsettle get events failed",
			logger.String("chain", chainName),
			logger.String("agent", agentID),
			logger.Error(err))
		return nil, 0, errno.ErrInternal
	}
	return events, total, nil
}

// GetScore returns the latest aggregated score for an agent.
func (s *PrismSettleService) GetScore(
	ctx context.Context, chainName, contractAddr, agentID string,
) (*model.ChainEvent, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, errno.ErrInvalidParam
	}

	ev, err := s.ceRepo.GetPrismScore(ctx, chainName, contractAddr, agentID)
	if err != nil {
		if err == errno.ErrNotFound {
			return nil, errno.ErrNotFound
		}
		logger.Warn("prismsettle get score failed",
			logger.String("agent", agentID),
			logger.Error(err))
		return nil, errno.ErrInternal
	}
	return ev, nil
}

// CountValidations returns the number of ValidationSubmitted events for an
// agent. Convenient for the demo dashboard.
func (s *PrismSettleService) CountValidations(
	ctx context.Context, chainName, contractAddr, agentID string,
) (int64, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return 0, errno.ErrInvalidParam
	}
	return s.ceRepo.CountPrismValidations(ctx, chainName, contractAddr, agentID)
}

// -----------------------------------------------------------------------------
// Phase 7 task 7.1: agent endpoints
// -----------------------------------------------------------------------------

// ListAgents returns agents ranked by reputation score (desc), tie-broken
// by completed task count (desc), then paginated. FR-A06. Ranking happens
// after score enrichment because scores are not stored in the DB.
func (s *PrismSettleService) ListAgents(
	ctx context.Context, chainName string, page, size int,
) ([]model.AgentVO, int64, error) {
	if s.agentRepo == nil {
		return nil, 0, errno.ErrInternal
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	recs, err := s.agentRepo.List(ctx, chainName)
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.AgentVO, 0, len(recs))
	for i := range recs {
		out = append(out, model.AgentVO{
			AgentID:      recs[i].AgentID,
			Owner:        recs[i].Owner,
			Metadata:     recs[i].Metadata,
			Endpoint:     recs[i].Endpoint,
			RegisteredAt: recs[i].RegisteredAt,
			BlockNumber:  recs[i].BlockNumber,
			Score:        "0",
		})
	}
	s.enrichScores(ctx, chainName, recs, out)
	s.enrichTaskCounts(ctx, chainName, out)

	// Rank: reputation desc → completed tasks desc → agentId (deterministic).
	sort.SliceStable(out, func(i, j int) bool {
		if c := cmpScore(out[i].Score, out[j].Score); c != 0 {
			return c > 0
		}
		if out[i].TaskCount != out[j].TaskCount {
			return out[i].TaskCount > out[j].TaskCount
		}
		return out[i].AgentID < out[j].AgentID
	})
	total := int64(len(out))

	lo := (page - 1) * size
	if lo >= len(out) {
		return []model.AgentVO{}, total, nil
	}
	hi := lo + size
	if hi > len(out) {
		hi = len(out)
	}
	return out[lo:hi], total, nil
}

// cmpScore compares two 1e18-scaled score strings numerically (-1/0/1).
// Unparseable strings are treated as zero so bad rows never break ranking.
func cmpScore(a, b string) int {
	ai, aok := new(big.Int).SetString(a, 10)
	bi, bok := new(big.Int).SetString(b, 10)
	if !aok {
		ai = big.NewInt(0)
	}
	if !bok {
		bi = big.NewInt(0)
	}
	return ai.Cmp(bi)
}

// enrichTaskCounts fills each AgentVO's TaskCount with the number of
// completed jobs whose provider is the agent's owner wallet.
func (s *PrismSettleService) enrichTaskCounts(ctx context.Context, chainName string, out []model.AgentVO) {
	for i := range out {
		n, err := s.ceRepo.CountCompletedJobs(ctx, chainName, out[i].Owner)
		if err != nil {
			logger.Warn("count completed jobs failed",
				logger.String("agent", out[i].AgentID), logger.Error(err))
			continue
		}
		out[i].TaskCount = n
	}
}

// enrichScores fills each AgentVO's Score: DB PRISM_AGGREGATED event first
// (fast, no RPC), then a parallel eth_call fallback for the rest. Parallelism
// matters — seed agents have no aggregation event and the testnet RPC
// rate-limits hard; serial fetches made /agents take seconds.
func (s *PrismSettleService) enrichScores(
	ctx context.Context,
	chainName string,
	recs []model.AgentRegistryRecord,
	out []model.AgentVO,
) {
	scores := make([]string, len(recs))
	needChain := make([]bool, len(recs))
	for i := range recs {
		if ev, err := s.ceRepo.GetPrismScore(ctx, chainName, "", recs[i].AgentID); err == nil {
			scores[i] = ev.Value
		} else {
			needChain[i] = true
		}
	}
	if s.scoreFetcher == nil {
		for i := range recs {
			out[i].Score = scores[i]
		}
		return
	}
	var wg sync.WaitGroup
	for i := range recs {
		if !needChain[i] {
			continue
		}
		wg.Add(1)
		go func(idx int, agentID string) {
			defer wg.Done()
			live, err := s.scoreFetcher.FetchScore(ctx, agentID)
			if err != nil {
				logger.Warn("score fetch failed", logger.String("agent", agentID), logger.Error(err))
				return
			}
			scores[idx] = live
		}(i, recs[i].AgentID)
	}
	wg.Wait()
	for i := range recs {
		out[i].Score = scores[i]
	}
}

// GetAgent returns a single agent by agentId, enriched with score. FR-A07.
func (s *PrismSettleService) GetAgent(
	ctx context.Context, chainName, agentID string,
) (*model.AgentVO, error) {
	if s.agentRepo == nil {
		return nil, errno.ErrInternal
	}
	rec, err := s.agentRepo.Get(ctx, chainName, agentID)
	if err != nil {
		return nil, err
	}
	vo := &model.AgentVO{
		AgentID:      rec.AgentID,
		Owner:        rec.Owner,
		Metadata:     rec.Metadata,
		Endpoint:     rec.Endpoint,
		RegisteredAt: rec.RegisteredAt,
		BlockNumber:  rec.BlockNumber,
		Score:        "0",
	}
	if ev, err := s.ceRepo.GetPrismScore(ctx, chainName, "", rec.AgentID); err == nil {
		vo.Score = ev.Value
	} else if s.scoreFetcher != nil {
		// Fallback: live on-chain score (seed agents have no aggregation
		// event). Same path as ListAgents, sharing its cache so the list
		// page and the detail page always agree.
		if live, err := s.scoreFetcher.FetchScore(ctx, rec.AgentID); err == nil {
			vo.Score = live
		}
	}
	if n, err := s.ceRepo.CountCompletedJobs(ctx, chainName, rec.Owner); err == nil {
		vo.TaskCount = n
	}
	return vo, nil
}

// -----------------------------------------------------------------------------
// Phase 7 task 7.1: jobs endpoints
// -----------------------------------------------------------------------------

// ListJobs returns paginated PrismSettleJob lifecycle events filtered by
// state. FR-A08.
func (s *PrismSettleService) ListJobs(
	ctx context.Context, chainName, contractAddr, state string,
	page, size int,
) ([]model.ChainEvent, int64, error) {
	return s.ceRepo.ListPrismJobs(ctx, chainName, contractAddr, state, page, size)
}

// GetJobStatus returns the latest lifecycle event for a jobId, representing
// its current state. FR-A09 / FR-A10.
func (s *PrismSettleService) GetJobStatus(
	ctx context.Context, chainName, jobID string,
) (*model.ChainEvent, error) {
	events, err := s.ceRepo.GetPrismJobTimeline(ctx, chainName, jobID)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, errno.ErrNotFound
	}
	// Timeline is ASC; last event is current state.
	return &events[len(events)-1], nil
}

// GetJobTimeline returns the chronological event list for a jobId. FR-JM02.
func (s *PrismSettleService) GetJobTimeline(
	ctx context.Context, chainName, jobID string,
) ([]model.ChainEvent, error) {
	return s.ceRepo.GetPrismJobTimeline(ctx, chainName, jobID)
}

// -----------------------------------------------------------------------------
// Phase 7 task 7.1: trust endpoint (FR-A11)
// Phase 7 task 7.2: threshold config (FR-AP11)
// -----------------------------------------------------------------------------

// CheckTrust evaluates an agent's score against the trust thresholds and
// returns ALLOW / DENY / REQUIRE_VALIDATION. FR-A11.
func (s *PrismSettleService) CheckTrust(
	ctx context.Context, chainName, contractAddr, agentID string,
) (*model.TrustResultVO, error) {
	if s.trustRepo == nil {
		return nil, errno.ErrInternal
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, errno.ErrInvalidParam
	}

	// Fetch latest score.
	ev, err := s.ceRepo.GetPrismScore(ctx, chainName, contractAddr, agentID)
	if err != nil {
		if err == errno.ErrNotFound {
			return &model.TrustResultVO{
				AgentID:  agentID,
				Score:    "0",
				Decision: model.TrustRequireValidation,
				Reason:   "no aggregation yet, defaulting to REQUIRE_VALIDATION",
			}, nil
		}
		return nil, err
	}

	threshold, err := s.trustRepo.Get(ctx, agentID)
	if err != nil {
		return nil, err
	}

	score, ok := new(big.Int).SetString(ev.Value, 10)
	if !ok {
		return nil, errno.ErrInternal
	}
	allow, ok := new(big.Int).SetString(threshold.AllowThreshold, 10)
	if !ok {
		return nil, errno.ErrInternal
	}
	deny, ok := new(big.Int).SetString(threshold.DenyThreshold, 10)
	if !ok {
		return nil, errno.ErrInternal
	}

	var decision model.TrustDecision
	var reason string
	switch {
	case score.Cmp(allow) >= 0:
		decision = model.TrustAllow
		reason = "score above allow threshold"
	case score.Cmp(deny) < 0:
		decision = model.TrustDeny
		reason = "score below deny threshold"
	default:
		decision = model.TrustRequireValidation
		reason = "score in validation band"
	}
	return &model.TrustResultVO{
		AgentID:  agentID,
		Score:    ev.Value,
		Decision: decision,
		Reason:   reason,
	}, nil
}

// SetTrustThreshold upserts a trust threshold row. FR-AP11.
func (s *PrismSettleService) SetTrustThreshold(
	ctx context.Context, agentID, allowThreshold, denyThreshold string,
) error {
	if s.trustRepo == nil {
		return errno.ErrInternal
	}
	// Validate numeric thresholds.
	if _, ok := new(big.Int).SetString(allowThreshold, 10); !ok {
		return errno.ErrInvalidParam
	}
	if _, ok := new(big.Int).SetString(denyThreshold, 10); !ok {
		return errno.ErrInvalidParam
	}
	return s.trustRepo.Upsert(ctx, agentID, allowThreshold, denyThreshold)
}

// -----------------------------------------------------------------------------
// Phase 7 task 7.1: reputation history (FR-A12)
// -----------------------------------------------------------------------------

// GetReputationHistory returns the most recent N aggregation/validation
// events for an agent. Default N=30. FR-A12.
func (s *PrismSettleService) GetReputationHistory(
	ctx context.Context, chainName, contractAddr, agentID string, limit int,
) ([]model.ChainEvent, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, errno.ErrInvalidParam
	}
	return s.ceRepo.GetPrismReputationHistory(ctx, chainName, contractAddr, agentID, limit)
}

// -----------------------------------------------------------------------------
// Phase 7 task 7.1: shards activity (FR-A04)
// -----------------------------------------------------------------------------

// GetShardActivity returns per-shard validation counts. FR-A04.
func (s *PrismSettleService) GetShardActivity(
	ctx context.Context, chainName, contractAddr string,
) ([]model.ShardActivityVO, error) {
	return s.ceRepo.GetPrismShardActivity(ctx, chainName, contractAddr)
}

// -----------------------------------------------------------------------------
// Phase 7 task 7.1: perf endpoints (mock data, Phase 9 replaces)
// -----------------------------------------------------------------------------

// GetPerfComparison returns V0/V1 benchmark comparison. Phase 9 reads the
// latest row from perf_results; if no benchmark has been run yet, it falls
// back to the Phase 7 mock values so the dashboard still renders. FR-T05.
func (s *PrismSettleService) GetPerfComparison(ctx context.Context) model.PerfComparisonVO {
	mock := model.PerfComparisonVO{
		V0AbortRate:  0.60, // ~60% per project memory NFR-MN01
		V1AbortRate:  0.03, // <5% per FR-T06
		V0Throughput: 120.0,
		V1Throughput: 480.0,
		MeetsFR_T06:  true,
		Source:       "mock",
	}

	if s.perfRepo == nil {
		return mock
	}
	rec, err := s.perfRepo.GetLatest(ctx)
	if err != nil || rec == nil {
		return mock
	}
	return model.PerfComparisonVO{
		V0AbortRate:  rec.V0AbortRate,
		V1AbortRate:  rec.V1AbortRate,
		V0Throughput: rec.V0Throughput,
		V1Throughput: rec.V1Throughput,
		MeetsFR_T06:  rec.MeetsFR_T06,
		Source:       "benchmark",
	}
}

// GetReorgFeed returns recent reorg events. Phase 9 reads from reorg_events;
// when no reorgs have been recorded yet, returns an empty list. FR-M07.
func (s *PrismSettleService) GetReorgFeed(ctx context.Context, chainName string, limit int) []model.ReorgFeedItemVO {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if s.reorgRepo == nil {
		return []model.ReorgFeedItemVO{}
	}
	recs, err := s.reorgRepo.ListRecent(ctx, chainName, limit)
	if err != nil || len(recs) == 0 {
		return []model.ReorgFeedItemVO{}
	}
	out := make([]model.ReorgFeedItemVO, 0, len(recs))
	for i := range recs {
		out = append(out, model.ReorgFeedItemVO{
			BlockNumber: recs[i].ToBlock,
			OldHash:     recs[i].OldHash,
			NewHash:     recs[i].NewHash,
			DetectedAt:  uint64(recs[i].DetectedAt.Unix()),
			Rollback:    recs[i].RollbackDepth > 0,
		})
	}
	return out
}

// -----------------------------------------------------------------------------
// Phase 7 task 7.1: agent invoke proxy (FR-M11)
// -----------------------------------------------------------------------------

// InvokeAgent proxies an A2A /invoke call to the target agent's endpoint.
// Implemented in the handler layer where the HTTP client lives.
