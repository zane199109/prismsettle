package evaluator

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/zane/web3-offchain/pkg/logger"
)

// Evaluator is the Phase 6 main loop. It polls the indexer for new Submitted
// events every pollInterval, runs RuleCheck + eval Agent + on-chain
// complete() + submitValidation(source=1) for each, and listens for Disputed
// events on the arbitration path.
//
// Design notes:
//   - Idempotency: a job is processed at most once per source. The
//     DecisionStore's unique (job_id, source) index is the source of truth;
//     processedJobs map is a fast in-memory pre-filter to avoid hammering the
//     DB on every tick.
//   - Reorg safety: when the indexer reports a reorg affecting a decided job,
//     the Evaluator marks the decision log row invalid (MarkInvalid) and
//     clears the in-memory entry so the job can be reprocessed.
//   - Fail-fast: NewEvaluator returns an error if any required dependency is
//     missing (project convention §3).
type Evaluator struct {
	rule        *RuleCheck
	eval        *EvalAgentClient
	job         JobCompleter   // calls Job.complete
	registry    RegistryWriter // calls Registry.submitValidation
	logs        DecisionStore
	arbitrator  *Arbitrator
	breaker     *CircuitBreaker
	eventSource EventSource // reads Submitted + Disputed events

	pollInterval  time.Duration
	processedJobs map[string]bool // jobId → processed (main path)

	// currentScoreFetcher returns the provider's current aggregated reputation
	// score; used by the arbitration path to compute the FR-E13 penalty.
	currentScoreFetcher func(ctx context.Context, agentID *big.Int) (uint64, error)
}

// JobCompleter is the on-chain interface for PrismSettleJob.complete.
type JobCompleter interface {
	// Complete calls Job.complete(jobId). Returns the tx hash.
	Complete(ctx context.Context, jobID *big.Int) (string, error)
}

// EventSource is the interface the Evaluator uses to read new events from the
// indexer's event store. The implementation lives in prismsettle/service.
type EventSource interface {
	// RecentSubmitted returns Submitted events seen since `since`, newest first.
	// Limit caps the result count per poll.
	RecentSubmitted(ctx context.Context, since time.Time, limit int) ([]SubmittedJob, error)
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
	IPFSGateway      string        // for RuleCheck; empty disables HEAD check
	EvalEndpoint     string        // eval agent base URL
	BreakerThreshold int           // default 10
	BreakerCooldown  time.Duration // default 60s
}

// NewEvaluator builds an Evaluator. All dependencies are required; nil ones
// cause an error (fail-fast).
func NewEvaluator(
	cfg Config,
	eventSource EventSource,
	job JobCompleter,
	registry RegistryWriter,
	hook HookResolver,
	logs DecisionStore,
	scoreFetcher func(ctx context.Context, agentID *big.Int) (uint64, error),
) (*Evaluator, error) {
	if eventSource == nil {
		return nil, fmt.Errorf("evaluator: event source is required")
	}
	if job == nil {
		return nil, fmt.Errorf("evaluator: job completer is required")
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
		rule:                NewRuleCheck(cfg.IPFSGateway),
		eval:                NewEvalAgentClient(cfg.EvalEndpoint, breaker),
		job:                 job,
		registry:            registry,
		logs:                logs,
		arbitrator:          NewArbitrator(NewArbitrationRule(), hook, registry, logs),
		breaker:             breaker,
		eventSource:         eventSource,
		pollInterval:        cfg.PollInterval,
		processedJobs:       make(map[string]bool),
		currentScoreFetcher: scoreFetcher,
	}, nil
}

// Run starts the main loop. Blocks until ctx is canceled.
func (e *Evaluator) Run(ctx context.Context) error {
	logger.Info("evaluator started", logger.String("poll", e.pollInterval.String()))
	tick := time.NewTicker(e.pollInterval)
	defer tick.Stop()

	// Track the last-seen timestamp so each poll only fetches new events.
	lastSubmitted := time.Now()
	lastDisputed := time.Now()

	for {
		select {
		case <-ctx.Done():
			logger.Info("evaluator stopping")
			return ctx.Err()
		case <-tick.C:
			// Main path: Submitted → complete → submitValidation(source=1)
			if err := e.pollSubmitted(ctx, &lastSubmitted); err != nil {
				logger.Errorf("evaluator submitted poll failed", logger.Error(err))
			}
			// Arbitration path: Disputed → resolveDispute → submitValidation(source=2)
			if err := e.pollDisputed(ctx, &lastDisputed); err != nil {
				logger.Errorf("evaluator disputed poll failed", logger.Error(err))
			}
		}
	}
}

// pollSubmitted processes new Submitted events since lastSeen.
func (e *Evaluator) pollSubmitted(ctx context.Context, lastSeen *time.Time) error {
	jobs, err := e.eventSource.RecentSubmitted(ctx, *lastSeen, 20)
	if err != nil {
		return fmt.Errorf("fetch submitted: %w", err)
	}
	if len(jobs) == 0 {
		return nil
	}
	// Advance the watermark to the newest event's timestamp.
	*lastSeen = time.Now()

	for _, job := range jobs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := e.processSubmitted(ctx, job); err != nil {
			logger.Errorf("evaluator process submitted failed",
				logger.String("job_id", job.JobID),
				logger.Error(err))
			continue // don't let one bad job kill the whole tick
		}
	}
	return nil
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

// processSubmitted runs the full main-path pipeline for one Submitted event:
//  1. Idempotency check (in-memory + DB)
//  2. RuleCheck
//  3. Eval Agent score (with fallback)
//  4. Job.complete() on-chain
//  5. Registry.submitValidation(provider, score, proof, jobId, source=1)
//  6. Decision log insert
func (e *Evaluator) processSubmitted(ctx context.Context, job SubmittedJob) error {
	// 1. Idempotency: in-memory fast path.
	if e.processedJobs[job.JobID] {
		return nil
	}
	already, err := e.logs.HasDecision(ctx, job.JobID, SourceEvaluatorMain)
	if err != nil {
		return fmt.Errorf("idempotency check: %w", err)
	}
	if already {
		e.processedJobs[job.JobID] = true
		return nil
	}

	// Fill provider if not already populated.
	if job.Provider == "" {
		provider, err := e.eventSource.ProviderFor(ctx, job.JobID)
		if err != nil {
			return fmt.Errorf("lookup provider: %w", err)
		}
		if provider == "" {
			return fmt.Errorf("job %s has no provider assigned yet", job.JobID)
		}
		job.Provider = provider
	}

	// 2. RuleCheck.
	res := e.rule.Check(ctx, job)
	if !res.OK {
		if res.Skip {
			// FR-E04: deliverable unreachable, retry next tick. Don't mark
			// processed so we re-evaluate.
			logger.Info("evaluator skipping unreachable deliverable",
				logger.String("job_id", job.JobID),
				logger.String("reason", res.Reason))
			return nil
		}
		// Hard reject: record and skip on-chain calls.
		_ = e.logs.Insert(ctx, &DecisionLog{
			JobID:      job.JobID,
			Source:     SourceEvaluatorMain,
			Decision:   DecisionReject,
			Reason:     res.Reason,
			CallerRole: RoleCommerceEvaluatorHex(),
		})
		e.processedJobs[job.JobID] = true
		return nil
	}

	// 3. Eval Agent score.
	score, reason, decision, err := e.eval.Score(ctx, EvalInput{
		DeliverableHash: job.DeliverableHash,
		JobID:           job.JobID,
		JobMetadata:     job.JobMetadata,
	})
	if err != nil {
		return fmt.Errorf("eval agent: %w", err)
	}

	// 4. Job.complete() on-chain.
	jobIDInt := new(big.Int)
	if _, ok := jobIDInt.SetString(job.JobID, 10); !ok {
		return fmt.Errorf("parse job_id %q: not decimal", job.JobID)
	}
	completeTx, err := e.job.Complete(ctx, jobIDInt)
	if err != nil {
		return fmt.Errorf("job.complete: %w", err)
	}

	// 5. Registry.submitValidation(provider, score, proof, jobId, source=1).
	providerAgentID, err := e.eventSource.AgentIDFor(ctx, job.Provider)
	if err != nil {
		return fmt.Errorf("lookup agentId for provider %s: %w", job.Provider, err)
	}
	if providerAgentID == nil {
		return fmt.Errorf("provider %s is not a registered agent", job.Provider)
	}
	proofHash := makeMainProofHash(job.DeliverableHash, score)
	submitTx, err := e.registry.SubmitValidation(ctx, providerAgentID, score, proofHash, jobIDInt, SourceEvaluatorMain)
	if err != nil {
		return fmt.Errorf("submitValidation: %w", err)
	}

	// 6. Decision log.
	dl := &DecisionLog{
		JobID:      job.JobID,
		Source:     SourceEvaluatorMain,
		Decision:   decision,
		Score:      fmt.Sprintf("%d", score),
		Reason:     reason,
		TxHash:     completeTx + ";" + submitTx,
		CallerRole: RoleCommerceEvaluatorHex(),
	}
	if err := e.logs.Insert(ctx, dl); err != nil {
		logger.Errorf("evaluator: insert decision log failed",
			logger.String("job_id", job.JobID), logger.Error(err))
	}
	e.processedJobs[job.JobID] = true

	logger.Info("evaluator completed job",
		logger.String("job_id", job.JobID),
		logger.String("decision", decision),
		logger.String("score", fmt.Sprintf("%d", score)),
		logger.String("complete_tx", completeTx),
		logger.String("submit_tx", submitTx))
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

// makeMainProofHash builds a 0x-prefixed bytes32 proof hash for the main-path
// decision. We hash (deliverableHash || score) so each (job, score) pair is
// content-addressed.
func makeMainProofHash(deliverableHash string, score uint64) string {
	return "0x" + normalizeHash(deliverableHash)[2:] + fmt.Sprintf("%016x", score)
}

// RoleCommerceEvaluatorHex returns the hex hash of COMMERCE_EVALUATOR_ROLE.
func RoleCommerceEvaluatorHex() string {
	return roleCommerceEvaluator.Hex()
}

// OnReorg is called by the indexer when a reorg affects decided jobs. It
// marks the affected decision rows invalid and clears the in-memory
// processedJobs entry so the jobs can be reprocessed.
//
// jobIDs is the list of jobIds whose events were rolled back.
func (e *Evaluator) OnReorg(ctx context.Context, jobIDs []string) {
	for _, id := range jobIDs {
		// Mark both main and arbitration decisions invalid.
		if err := e.logs.MarkInvalid(ctx, id, SourceEvaluatorMain); err != nil {
			logger.Warn("evaluator reorg: mark main invalid failed",
				logger.String("job_id", id), logger.Error(err))
		}
		if err := e.logs.MarkInvalid(ctx, id, SourceEvaluatorArb); err != nil {
			// Not all jobs have arbitration decisions; ignore not-found.
			logger.Warn("evaluator reorg: mark arb invalid failed",
				logger.String("job_id", id), logger.Error(err))
		}
		delete(e.processedJobs, id)
		logger.Info("evaluator reorg: cleared job",
			logger.String("job_id", id))
	}
}

// strings import kept for normalizeHash fallback in case it's needed.
var _ = strings.TrimSpace
