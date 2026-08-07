package evaluator

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/zane/web3-offchain/pkg/logger"
)

// Evaluator is the Phase 6 arbitration loop. It polls the indexer for
// Disputed events, evaluates the deliverable, and resolves the dispute
// via ArbitrationHook.resolveDispute + Registry.setAggregatedScore.
//
// The main path (submit → complete + submitValidation source=1) is now
// handled by the Buyer directly via PrismSettleJob.complete(score).
// The Evaluator only intervenes when a dispute is raised.
//
// Design notes:
//   - Idempotency: a job is processed at most once per source. The
//     DecisionStore's unique (job_id, source) index is the source of truth.
//   - Reorg safety: when the indexer reports a reorg affecting a decided job,
//     the Evaluator marks the decision log row invalid (MarkInvalid).
//   - Fail-fast: NewEvaluator returns an error if any required dependency is
//     missing (project convention §3).
type Evaluator struct {
	eval        *EvalAgentClient
	registry    RegistryWriter // calls Registry.submitValidation / setAggregatedScore
	logs        DecisionStore
	arbitrator  *Arbitrator
	breaker     *CircuitBreaker
	eventSource EventSource // reads Disputed events

	pollInterval time.Duration

	// currentScoreFetcher returns the provider's current aggregated reputation
	// score; used by the arbitration path to compute the FR-E13 penalty.
	currentScoreFetcher func(ctx context.Context, agentID *big.Int) (uint64, error)
}

// EventSource is the interface the Evaluator uses to read new events from the
// indexer's event store. The implementation lives in prismsettle/service.
type EventSource interface {
	// RecentDisputed returns Disputed events seen since `since`, newest first.
	RecentDisputed(ctx context.Context, since time.Time, limit int) ([]DisputedJob, error)
	// ProviderFor returns the provider address for a job (looked up from
	// Assigned event). Empty string means "unknown / not yet assigned".
	ProviderFor(ctx context.Context, jobID string) (string, error)
	// AgentIDFor returns the Registry agentId for a provider address. The
	// Evaluator needs this to call submitValidation.
	AgentIDFor(ctx context.Context, providerAddr string) (*big.Int, error)
	// CurrentScore returns the provider's latest aggregated reputation score.
	CurrentScore(ctx context.Context, agentID *big.Int) (uint64, error)
}

// Config holds Evaluator tuning knobs.
type Config struct {
	PollInterval     time.Duration // default 2s (FR-E01)
	BatchSize        int           // max events per poll; default 20
	EvalEndpoint     string        // eval agent base URL
	BreakerThreshold int           // default 10
	BreakerCooldown  time.Duration // default 60s
}

// NewEvaluator builds an Evaluator. All dependencies are required; nil ones
// cause an error (fail-fast).
func NewEvaluator(
	cfg Config,
	eventSource EventSource,
	registry RegistryWriter,
	hook HookResolver,
	logs DecisionStore,
	scoreFetcher func(ctx context.Context, agentID *big.Int) (uint64, error),
) (*Evaluator, error) {
	if eventSource == nil {
		return nil, fmt.Errorf("evaluator: event source is required")
	}
	if registry == nil {
		return nil, fmt.Errorf("evaluator: registry writer is required")
	}
	if hook == nil {
		return nil, fmt.Errorf("evaluator: hook resolver is required")
	}
	if logs == nil {
		return nil, fmt.Errorf("evaluator: decision store is required")
	}
	if scoreFetcher == nil {
		return nil, fmt.Errorf("evaluator: score fetcher is required")
	}

	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second // FR-E01
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 20
	}

	breaker := NewCircuitBreaker(cfg.BreakerThreshold, cfg.BreakerCooldown)
	return &Evaluator{
		eval:                NewEvalAgentClient(cfg.EvalEndpoint, breaker),
		registry:            registry,
		logs:                logs,
		arbitrator:          NewArbitrator(NewArbitrationRule(), hook, registry, logs),
		breaker:             breaker,
		eventSource:         eventSource,
		pollInterval:        cfg.PollInterval,
		currentScoreFetcher: scoreFetcher,
	}, nil
}

// Run starts the arbitration loop. Blocks until ctx is canceled.
func (e *Evaluator) Run(ctx context.Context) error {
	logger.Info("evaluator started (arbitration only)", logger.String("poll", e.pollInterval.String()))
	tick := time.NewTicker(e.pollInterval)
	defer tick.Stop()

	lastDisputed := time.Now()

	for {
		select {
		case <-ctx.Done():
			logger.Info("evaluator stopping")
			return ctx.Err()
		case <-tick.C:
			// Arbitration path: Disputed → resolveDispute → setAggregatedScore(source=2)
			if err := e.pollDisputed(ctx, &lastDisputed); err != nil {
				logger.Errorf("evaluator disputed poll failed", logger.Error(err))
			}
		}
	}
}

// pollDisputed processes new Disputed events since lastSeen.
func (e *Evaluator) pollDisputed(ctx context.Context, lastSeen *time.Time) error {
	jobs, err := e.eventSource.RecentDisputed(ctx, *lastSeen, 20)
	if err != nil {
		return fmt.Errorf("fetch disputed: %w", err)
	}
	if len(jobs) == 0 {
		return nil
	}
	*lastSeen = time.Now()

	for _, job := range jobs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := e.processDisputed(ctx, job); err != nil {
			logger.Errorf("evaluator process disputed failed",
				logger.String("job_id", job.JobID),
				logger.Error(err))
			continue
		}
	}
	return nil
}

// processDisputed runs the arbitration pipeline for one Disputed event.
func (e *Evaluator) processDisputed(ctx context.Context, job DisputedJob) error {
	// Idempotency.
	already, err := e.logs.HasDecision(ctx, job.JobID, SourceEvaluatorArb)
	if err != nil {
		return fmt.Errorf("arb idempotency check: %w", err)
	}
	if already {
		return nil
	}

	// Look up provider + agentId.
	if job.Provider == "" {
		provider, err := e.eventSource.ProviderFor(ctx, job.JobID)
		if err != nil {
			return fmt.Errorf("arb lookup provider: %w", err)
		}
		job.Provider = provider
	}
	providerAgentID, err := e.eventSource.AgentIDFor(ctx, job.Provider)
	if err != nil {
		return fmt.Errorf("arb lookup agentId: %w", err)
	}
	if providerAgentID == nil {
		return fmt.Errorf("arb: provider %s not registered", job.Provider)
	}

	// Fetch eval score (re-evaluate the deliverable for arbitration evidence).
	evalScore, _, _, err := e.eval.Score(ctx, EvalInput{
		DeliverableHash: job.DeliverableHash,
		JobID:           job.JobID,
		JobMetadata:     "",
	})
	if err != nil {
		return fmt.Errorf("arb eval score: %w", err)
	}
	currentScore, err := e.currentScoreFetcher(ctx, providerAgentID)
	if err != nil {
		return fmt.Errorf("arb current score: %w", err)
	}

	return e.arbitrator.Decide(ctx, job, providerAgentID, evalScore, currentScore)
}

// OnReorg is called by the indexer when a reorg affects decided jobs. It
// marks the affected decision rows invalid so the jobs can be reprocessed.
//
// jobIDs is the list of jobIds whose events were rolled back.
func (e *Evaluator) OnReorg(ctx context.Context, jobIDs []string) {
	for _, id := range jobIDs {
		if err := e.logs.MarkInvalid(ctx, id, SourceEvaluatorArb); err != nil {
			// Not all jobs have arbitration decisions; ignore not-found.
			logger.Warn("evaluator reorg: mark arb invalid failed",
				logger.String("job_id", id), logger.Error(err))
		}
		logger.Info("evaluator reorg: cleared job",
			logger.String("job_id", id))
	}
}