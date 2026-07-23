package evaluator

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"
)

// ===== RuleCheck tests (FR-E02 ~ FR-E05) =====

func TestRuleCheck_AllPass(t *testing.T) {
	rc := NewRuleCheck("") // no IPFS gateway → reachability skipped
	job := SubmittedJob{
		JobID:           "1",
		Provider:        "0xABC",
		Submitter:       "0xABC",
		DeliverableHash: "0x" + repeat("ab", 32),
		ProofHash:       "0x" + repeat("cd", 32),
	}
	res := rc.Check(context.Background(), job)
	if !res.OK {
		t.Fatalf("expected OK, got %s (skip=%v)", res.Reason, res.Skip)
	}
}

func TestRuleCheck_EmptyDeliverable(t *testing.T) {
	rc := NewRuleCheck("")
	job := SubmittedJob{JobID: "1", Provider: "0xABC", Submitter: "0xABC"}
	res := rc.Check(context.Background(), job)
	if res.OK {
		t.Fatal("expected failure on empty deliverable")
	}
	if res.Skip {
		t.Fatal("empty deliverable should reject, not skip")
	}
}

func TestRuleCheck_SubmitterMismatch(t *testing.T) {
	rc := NewRuleCheck("")
	job := SubmittedJob{
		JobID:           "1",
		Provider:        "0xABC",
		Submitter:       "0xDEF",
		DeliverableHash: "0x" + repeat("ab", 32),
		ProofHash:       "0x" + repeat("cd", 32),
	}
	res := rc.Check(context.Background(), job)
	if res.OK {
		t.Fatal("expected failure on submitter mismatch")
	}
}

func TestRuleCheck_ZeroProofHash(t *testing.T) {
	rc := NewRuleCheck("")
	job := SubmittedJob{
		JobID:           "1",
		Provider:        "0xABC",
		Submitter:       "0xABC",
		DeliverableHash: "0x" + repeat("ab", 32),
		ProofHash:       "0x" + repeat("0", 32),
	}
	res := rc.Check(context.Background(), job)
	if res.OK {
		t.Fatal("expected failure on zero proofHash")
	}
}

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

// ===== Evaluator idempotency + main path =====
//
// We drive the Evaluator with fake implementations of every dependency so we
// can assert on side effects without a live chain or LLM.

// fakeEventSource feeds pre-seeded jobs and tracks calls.
type fakeEventSource struct {
	mu        sync.Mutex
	submitted []SubmittedJob
	disputed  []DisputedJob
	providers map[string]string // jobID → provider
	agentIDs  map[string]string // provider → agentID
	scores    map[string]uint64 // agentID → current score

	submittedCalls int
	disputedCalls  int
}

func (f *fakeEventSource) RecentSubmitted(ctx context.Context, since time.Time, limit int) ([]SubmittedJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submittedCalls++
	return f.submitted, nil
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

// fakeJobCompleter records complete() calls.
type fakeJobCompleter struct {
	mu     sync.Mutex
	calls  []string
	txHash string
}

func (f *fakeJobCompleter) Complete(ctx context.Context, jobID *big.Int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, jobID.String())
	return f.txHash, nil
}

// fakeRegistryWriter records submitValidation() calls.
type fakeRegistryWriter struct {
	mu     sync.Mutex
	calls  []fakeSubmit
	txHash string
}
type fakeSubmit struct {
	AgentID   string
	Score     uint64
	ProofHash string
	JobID     string
	Source    uint8
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

func TestEvaluator_MainPathIdempotency(t *testing.T) {
	src := &fakeEventSource{
		submitted: []SubmittedJob{{
			JobID:           "1",
			Provider:        "0xABC",
			Submitter:       "0xABC",
			DeliverableHash: "0x" + repeat("ab", 32),
			ProofHash:       "0x" + repeat("cd", 32),
		}},
		providers: map[string]string{"1": "0xABC"},
		agentIDs:  map[string]string{"0xABC": "100"},
		scores:    map[string]uint64{"100": 500_000_000_000_000_000},
	}
	job := &fakeJobCompleter{txHash: "0xJOB1"}
	reg := &fakeRegistryWriter{txHash: "0xREG1"}
	hook := &fakeHookResolver{txHash: "0xHOOK1"}
	logs := &memoryDecisionStore{}

	// EvalAgentClient points at a dead port → fallback score 0.6e18.
	e, err := NewEvaluator(Config{PollInterval: 50 * time.Millisecond}, src, job, reg, hook, logs,
		func(ctx context.Context, id *big.Int) (uint64, error) { return 0, nil })
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = e.Run(ctx)

	// 1. complete() called exactly once (idempotency).
	if len(job.calls) != 1 {
		t.Errorf("expected 1 complete call, got %d", len(job.calls))
	}
	// 2. submitValidation called exactly once with source=1.
	if len(reg.calls) != 1 {
		t.Fatalf("expected 1 submitValidation call, got %d", len(reg.calls))
	}
	if reg.calls[0].Source != SourceEvaluatorMain {
		t.Errorf("expected source=1, got %d", reg.calls[0].Source)
	}
	// 3. Fallback score recorded.
	if reg.calls[0].Score != FallbackScore {
		t.Errorf("expected fallback score, got %d", reg.calls[0].Score)
	}
	// 4. Decision log inserted.
	if len(logs.logs) != 1 {
		t.Fatalf("expected 1 decision log, got %d", len(logs.logs))
	}
	if logs.logs[0].Source != SourceEvaluatorMain {
		t.Errorf("expected source=1 log, got %d", logs.logs[0].Source)
	}
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
	job := &fakeJobCompleter{}
	reg := &fakeRegistryWriter{txHash: "0xREG2"}
	hook := &fakeHookResolver{txHash: "0xHOOK2"}
	logs := &memoryDecisionStore{}

	e, err := NewEvaluator(Config{PollInterval: 50 * time.Millisecond}, src, job, reg, hook, logs,
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
	// 2. submitValidation called once with source=2.
	if len(reg.calls) != 1 {
		t.Fatalf("expected 1 submitValidation call, got %d", len(reg.calls))
	}
	if reg.calls[0].Source != SourceEvaluatorArb {
		t.Errorf("expected source=2, got %d", reg.calls[0].Source)
	}
	// 3. Penalty score = max(0.2e18, 1e18*30%) = 0.3e18.
	if reg.calls[0].Score != 300_000_000_000_000_000 {
		t.Errorf("expected 0.3e18 penalty, got %d", reg.calls[0].Score)
	}
}

func TestEvaluator_ReorgClearsProcessedJobs(t *testing.T) {
	logs := &memoryDecisionStore{}
	logs.Insert(context.Background(), &DecisionLog{
		JobID:    "99",
		Source:   SourceEvaluatorMain,
		Decision: DecisionComplete,
	})
	src := &fakeEventSource{providers: map[string]string{}, agentIDs: map[string]string{}, scores: map[string]uint64{}}
	e, err := NewEvaluator(Config{PollInterval: 50 * time.Millisecond}, src,
		&fakeJobCompleter{}, &fakeRegistryWriter{}, &fakeHookResolver{}, logs,
		func(ctx context.Context, id *big.Int) (uint64, error) { return 0, nil })
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}

	// Mark processed, then reorg should clear it.
	e.processedJobs["99"] = true
	e.OnReorg(context.Background(), []string{"99"})
	if e.processedJobs["99"] {
		t.Fatal("reorg should clear processedJobs entry")
	}
	has, _ := logs.HasDecision(context.Background(), "99", SourceEvaluatorMain)
	if has {
		t.Fatal("reorg should mark decision invalid")
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
