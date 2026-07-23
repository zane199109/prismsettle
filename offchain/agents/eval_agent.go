package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zane/web3-offchain/pkg/logger"
)

// EvalAgent scores deliverables on a 0..1e18 scale (FR-E14). It wraps an
// internal LLM (default gpt-4o-mini) but exposes a stable interface to the
// Evaluator: same deliverable_hash always yields the same score, satisfying
// the variance < 0.05 requirement.
//
// Determinism strategy (in priority order):
//  1. In-memory cache: same deliverable_hash returns the cached score.
//  2. LLM temperature=0: identical prompts yield identical completions.
//  3. Output clamping: scores outside [0, 1e18] are clamped.
//
// The cache is keyed by deliverable_hash (not input) so the same deliverable
// scored from different job contexts still returns the same score.
type EvalAgent struct {
	BaseAgent
	cache sync.Map // deliverableHash (0x...) -> cachedEval
}

// cachedEval stores a single evaluation result for cache hits.
type cachedEval struct {
	score  uint64
	reason string
}

// evalInput is the expected shape of the /invoke input JSON (FR-E14).
type evalInput struct {
	DeliverableHash string `json:"deliverable_hash"`
	JobID           string `json:"job_id"`
	JobMetadata     string `json:"job_metadata"`
}

// evalOutput is the response shape returned by /invoke.
type evalOutput struct {
	Score  uint64 `json:"score"` // [0, 1e18]
	Reason string `json:"reason"`
}

// llmEvalResponse is the JSON we ask the LLM to produce.
type llmEvalResponse struct {
	Score  uint64 `json:"score"`
	Reason string `json:"reason"`
}

// NewEvalAgent builds the evaluation agent. The LLM client is shared with the
// business agents in the same process.
func NewEvalAgent(llm *LLMClient) *EvalAgent {
	return &EvalAgent{
		BaseAgent: NewBaseAgent(
			"eval",
			"Evaluates deliverable quality on a 0..1e18 scale. Deterministic: same deliverable_hash yields same score.",
			[]string{"evaluation", "quality-scoring"},
			llm,
		),
	}
}

// Invoke runs the evaluation agent. input must be a JSON object matching
// evalInput. Output is a JSON object matching evalOutput.
func (a *EvalAgent) Invoke(input, caller string) (string, string, error) {
	var in evalInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", "", fmt.Errorf("eval: parse input (expected JSON {deliverable_hash, job_id, job_metadata}): %w", err)
	}
	if in.DeliverableHash == "" {
		return "", "", fmt.Errorf("eval: deliverable_hash is required")
	}
	if !strings.HasPrefix(in.DeliverableHash, "0x") || len(in.DeliverableHash) != 66 {
		return "", "", fmt.Errorf("eval: deliverable_hash must be 0x-prefixed bytes32, got %q", in.DeliverableHash)
	}

	// 1. Cache hit -> deterministic.
	if cached, ok := a.cache.Load(in.DeliverableHash); ok {
		ce := cached.(cachedEval)
		logger.Info("eval cache hit",
			logger.String("deliverable_hash", in.DeliverableHash),
			logger.String("score", FormatScore(ce.score)))
		return a.formatOutput(ce)
	}

	// 2. Cache miss -> ask the LLM.
	cfg, err := CurrentConfig()
	if err != nil {
		return "", "", fmt.Errorf("eval: load config: %w", err)
	}
	prompt := cfg.Eval.SystemPrompt
	if prompt == "" {
		return "", "", fmt.Errorf("eval: system_prompt not configured")
	}
	model := cfg.Eval.Model
	timeout := time.Duration(cfg.LLM.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	userMsg, _ := json.Marshal(map[string]string{
		"deliverable_hash": in.DeliverableHash,
		"job_id":           in.JobID,
		"job_metadata":     in.JobMetadata,
	})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	raw, err := a.llm.Chat(ctx, cfg, prompt, string(userMsg), model)
	if err != nil {
		return "", "", fmt.Errorf("eval: llm chat: %w", err)
	}

	// 3. Parse + clamp + cache.
	le, err := parseLLMEvalResponse(raw)
	if err != nil {
		return "", "", fmt.Errorf("eval: parse llm response: %w (raw=%s)", err, raw)
	}
	if le.Score > 1e18 {
		le.Score = 1e18
	}
	ce := cachedEval{score: le.Score, reason: le.Reason}
	a.cache.Store(in.DeliverableHash, ce)
	logger.Info("eval scored",
		logger.String("deliverable_hash", in.DeliverableHash),
		logger.String("score", FormatScore(ce.score)),
		logger.String("caller", caller))
	return a.formatOutput(ce)
}

// formatOutput builds the JSON output string and its proof hash.
func (a *EvalAgent) formatOutput(ce cachedEval) (string, string, error) {
	out := evalOutput{Score: ce.score, Reason: ce.reason}
	body, err := json.Marshal(out)
	if err != nil {
		return "", "", fmt.Errorf("eval: marshal output: %w", err)
	}
	return string(body), ProofHash(string(body)), nil
}

// Metadata returns the agent.json descriptor.
func (a *EvalAgent) Metadata() AgentMetadata {
	cfg, err := CurrentConfig()
	endpoint := ""
	if err == nil {
		endpoint = cfg.Eval.Endpoint
	}
	return AgentMetadata{
		Name:         a.name,
		Description:  a.description,
		Version:      "1.0.0",
		Endpoint:     endpoint,
		Capabilities: a.capabilities,
	}
}

// parseLLMEvalResponse extracts the {score, reason} JSON from an LLM reply.
// Tolerates leading/trailing prose and code fences so a slightly misbehaving
// model still parses.
func parseLLMEvalResponse(raw string) (llmEvalResponse, error) {
	// Strip common code fences.
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// Find the first { ... } block.
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < 0 || end <= start {
		return llmEvalResponse{}, fmt.Errorf("no JSON object found in response")
	}
	body := s[start : end+1]

	var le llmEvalResponse
	if err := json.Unmarshal([]byte(body), &le); err != nil {
		return llmEvalResponse{}, fmt.Errorf("unmarshal: %w", err)
	}
	if le.Reason == "" {
		return llmEvalResponse{}, fmt.Errorf("reason field is empty")
	}
	return le, nil
}
