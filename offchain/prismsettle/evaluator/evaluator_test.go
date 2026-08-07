package evaluator

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"
)

// ===== Circuit Breaker tests =====

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)
	for i := 0; i < 3; i++ {
		cb.Failure()
	}
	if cb.State() != BreakerOpen {
		t.Fatalf("expected OPEN after 3 failures, got %d", cb.State())
	}
	if cb.Allow() {
		t.Fatal("OPEN breaker should not allow requests")
	}
}

func TestCircuitBreaker_OpenToHalfOpenToClosed(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	cb.Failure() // → OPEN
	time.Sleep(60 * time.Millisecond)
	if cb.State() != BreakerHalfOpen {
		t.Fatalf("expected HALF_OPEN after cooldown, got %d", cb.State())
	}
	if !cb.Allow() {
		t.Fatal("HALF_OPEN should allow one probe")
	}
	if cb.Allow() {
		t.Fatal("HALF_OPEN should not allow a second concurrent probe")
	}
	cb.Success()
	if cb.State() != BreakerClosed {
		t.Fatalf("expected CLOSED after probe success, got %d", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenReopensOnFailure(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	cb.Failure()
	time.Sleep(60 * time.Millisecond)
	_ = cb.Allow()
	cb.Failure()
	if cb.State() != BreakerOpen {
		t.Fatalf("expected OPEN after half-open failure, got %d", cb.State())
	}
}

func TestCircuitBreaker_WithBreaker(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)
	calls := 0
	err := cb.WithBreaker(context.Background(), func(ctx context.Context) error {
		calls++
		return nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("expected success, got err=%v calls=%d", err, calls)
	}
}

// ===== Arbitration rule tests (FR-E09) =====

func TestArbitrationRule_ThreePaths(t *testing.T) {
	r := NewArbitrationRule()
	deliverable := "0x" + repeat("ab", 32)

	// Path 1: reasonHash == 0 → provider wins (ruling=2)
	ruling, _ := r.Ruling(deliverable, "0x"+repeat("0", 32))
	if ruling != 2 {
		t.Errorf("path1: expected ruling=2, got %d", ruling)
	}

	// Path 2: reasonHash == deliverableHash → buyer wins (ruling=1)
	ruling, _ = r.Ruling(deliverable, deliverable)
	if ruling != 1 {
		t.Errorf("path2: expected ruling=1, got %d", ruling)
	}

	// Path 3: reasonHash != deliverableHash → provider wins (ruling=2)
	ruling, _ = r.Ruling(deliverable, "0x"+repeat("ef", 32))
	if ruling != 2 {
		t.Errorf("path3: expected ruling=2, got %d", ruling)
	}
}

func TestArbitrationScore_PenaltyFloor(t *testing.T) {
	// ruling=1, currentScore=0 → penalty = max(0.2e18, 0) = 0.2e18
	score, _ := ArbitrationScore(1, 0, 0)
	if score != 200_000_000_000_000_000 {
		t.Errorf("expected 0.2e18 floor, got %d", score)
	}
}

func TestArbitrationScore_ProportionalPenalty(t *testing.T) {
	// ruling=1, currentScore=1e18 → penalty = max(0.2e18, 1e18*30%) = 0.3e18
	score, _ := ArbitrationScore(1, 0, 1_000_000_000_000_000_000)
	if score != 300_000_000_000_000_000 {
		t.Errorf("expected 0.3e18 proportional, got %d", score)
	}
}

func TestArbitrationScore_ProviderWins(t *testing.T) {
	// ruling=2, evalScore=0.8e18 → score = 0.8e18
	score, _ := ArbitrationScore(2, 800_000_000_000_000_000, 0)
	if score != 800_000_000_000_000_000 {
		t.Errorf("expected evalScore, got %d", score)
	}
}

// ===== EvalAgentClient fallback tests (FR-E11) =====

func TestEvalAgentClient_FallbackOnUnreachable(t *testing.T) {
	// Point at a port nothing's listening on → connection refused → fallback.
	c := NewEvalAgentClient("http://127.0.0.1:65535", nil)
	score, reason, decision, err := c.Score(context.Background(), EvalInput{
		DeliverableHash: "0x" + repeat("ab", 32),
		JobID:           "1",
	})
	if err != nil {
		t.Fatalf("expected nil err on fallback, got %v", err)
	}
	if score != FallbackScore {
		t.Errorf("expected fallback score %d, got %d", FallbackScore, score)
	}
	if decision != DecisionFallback {
		t.Errorf("expected decision=fallback, got %s", decision)
	}
	if reason == "" {
		t.Error("fallback reason should not be empty")
	}
}

// ===== Evaluator arbitration path tests =====
//
// We drive the Evaluator with fake implementations of every dependency so we
// can assert on side effects without a live chain or LLM.

// fakeEventSource feeds pre-seeded jobs and tracks calls.
type fakeEventSource struct {
	mu        sync.Mutex
	disputed  []DisputedJob
	providers map[string]string // jobID → provider
	agentIDs  map[string]string // provider → agentID
	scores    map[string]uint64 // agentID → current score

	disputedCalls int
}

func (f *fakeEventSource) RecentDisputed(ctx context.Context, since time.Time, limit int) ([]DisputedJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disputedCalls++
	return f.disputed, nil
}
func (f *fakeEventSource) ProviderFor(ctx context.Context, jobID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.providers[jobID], nil
}
func (f *fakeEventSource) AgentIDFor(ctx context.Context, provider string) (*big.Int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.agentIDs[provider]
	if id == "" {
		return nil, nil
	}
	n, ok := new(big.Int).SetString(id, 10)
	if !ok {
		return nil, fmt.Errorf("bad agentID %q", id)
	}
	return n, nil
}
func (f *fakeEventSource) CurrentScore(ctx context.Context, agentID *big.Int) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.scores[agentID.String()], nil
}

// fakeRegistryWriter records submitValidation() and setAggregatedScore() calls.
type fakeRegistryWriter struct {
	mu     sync.Mutex
	calls  []fakeSubmit
	txHash string
	// aggCalls records setAggregatedScore invocations.
	aggCalls []fakeAggScore
}
type fakeSubmit struct {
	AgentID   string
	Score     uint64
	ProofHash string
	JobID     string
	Source    uint8
}
type fakeAggScore struct {
	AgentID string
	Score   uint64
}

func (f *fakeRegistryWriter) SubmitValidation(ctx context.Context, agentID *big.Int, score uint64, proofHash string, jobID *big.Int, source uint8) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeSubmit{
		AgentID:   agentID.String(),
		Score:     score,
		ProofHash: proofHash,
		JobID:     jobID.String(),
		Source:    source,
	})
	return f.txHash, nil
}

func (f *fakeRegistryWriter) SetAggregatedScore(ctx context.Context, agentID *big.Int, newScore uint64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.aggCalls = append(f.aggCalls, fakeAggScore{
		AgentID: agentID.String(),
		Score:   newScore,
	})
	return f.txHash, nil
}

// fakeHookResolver records resolveDispute() calls.
type fakeHookResolver struct {
	mu     sync.Mutex
	calls  []fakeResolve
	txHash string
}
type fakeResolve struct {
	JobID  string
	Ruling uint8
}

func (f *fakeHookResolver) ResolveDispute(ctx context.Context, jobID *big.Int, ruling uint8) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeResolve{JobID: jobID.String(), Ruling: ruling})
	return f.txHash, nil
}

// memoryDecisionStore is an in-memory DecisionStore for tests.
type memoryDecisionStore struct {
	mu   sync.Mutex
	logs []DecisionLog
}

func (m *memoryDecisionStore) Insert(ctx context.Context, d *DecisionLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Idempotency: reject duplicates on (job_id, source).
	for _, l := range m.logs {
		if l.JobID == d.JobID && l.Source == d.Source {
			return fmt.Errorf("duplicate")
		}
	}
	m.logs = append(m.logs, *d)
	return nil
}
func (m *memoryDecisionStore) HasDecision(ctx context.Context, jobId string, source uint8) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, l := range m.logs {
		if l.JobID == jobId && l.Source == source && !l.Invalid {
			return true, nil
		}
	}
	return false, nil
}
func (m *memoryDecisionStore) MarkInvalid(ctx context.Context, jobId string, source uint8) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.logs {
		if m.logs[i].JobID == jobId && m.logs[i].Source == source {
			m.logs[i].Invalid = true
		}
	}
	return nil
}
func (m *memoryDecisionStore) Recent(ctx context.Context, limit int) ([]DecisionLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs, nil
}

func TestEvaluator_ArbPath(t *testing.T) {
	deliverable := "0x" + repeat("ab", 32)
	src := &fakeEventSource{
		disputed: []DisputedJob{{
			JobID:           "2",
			Provider:        "0xABC",
			Buyer:           "0xDEF",
			DeliverableHash: deliverable,
			ReasonHash:      deliverable, // path 2: buyer wins (ruling=1)
		}},
		providers: map[string]string{"2": "0xABC"},
		agentIDs:  map[string]string{"0xABC": "200"},
		scores:    map[string]uint64{"200": 1_000_000_000_000_000_000},
	}
	reg := &fakeRegistryWriter{txHash: "0xREG2"}
	hook := &fakeHookResolver{txHash: "0xHOOK2"}
	logs := &memoryDecisionStore{}

	e, err := NewEvaluator(Config{PollInterval: 50 * time.Millisecond}, src, reg, hook, logs,
		func(ctx context.Context, id *big.Int) (uint64, error) {
			return 1_000_000_000_000_000_000, nil
		})
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = e.Run(ctx)

	// 1. resolveDispute called with ruling=1 (buyer wins).
	if len(hook.calls) != 1 {
		t.Fatalf("expected 1 resolveDispute call, got %d", len(hook.calls))
	}
	if hook.calls[0].Ruling != 1 {
		t.Errorf("expected ruling=1 (buyer wins), got %d", hook.calls[0].Ruling)
	}
	// 2. setAggregatedScore called once.
	if len(reg.aggCalls) != 1 {
		t.Fatalf("expected 1 setAggregatedScore call, got %d", len(reg.aggCalls))
	}
	// 3. Penalty score = max(0.2e18, 1e18*30%) = 0.3e18.
	if reg.aggCalls[0].Score != 300_000_000_000_000_000 {
		t.Errorf("expected 0.3e18 penalty, got %d", reg.aggCalls[0].Score)
	}
}

func TestEvaluator_ReorgClearsProcessedJobs(t *testing.T) {
	logs := &memoryDecisionStore{}
	logs.Insert(context.Background(), &DecisionLog{
		JobID:    "99",
		Source:   SourceEvaluatorArb,
		Decision: DecisionDisputeResolved,
	})
	src := &fakeEventSource{providers: map[string]string{}, agentIDs: map[string]string{}, scores: map[string]uint64{}}
	e, err := NewEvaluator(Config{PollInterval: 50 * time.Millisecond}, src,
		&fakeRegistryWriter{}, &fakeHookResolver{}, logs,
		func(ctx context.Context, id *big.Int) (uint64, error) { return 0, nil })
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}

	// Reorg should mark the arbitration decision as invalid.
	e.OnReorg(context.Background(), []string{"99"})
	has, _ := logs.HasDecision(context.Background(), "99", SourceEvaluatorArb)
	if has {
		t.Fatal("reorg should mark arbitration decision invalid")
	}
}

// repeat is a tiny helper to build n-char hex strings without fmt in loops.
func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
