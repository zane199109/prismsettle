package evaluator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EvalAgentClient calls the Phase 5 evaluation Agent over HTTP. It is the
// single egress point from the Evaluator to the LLM-backed eval service.
//
// Failure handling (FR-E11):
//   - timeout / network error / non-2xx → fallback score 0.6e18
//   - malformed JSON → fallback score 0.6e18
//   - score out of [0, 1e18] → clamp
//
// The fallback is recorded in decision_logs with decision="fallback" so
// operators can see when the LLM was bypassed.
type EvalAgentClient struct {
	endpoint   string // e.g. http://localhost:8004
	httpClient *http.Client
	breaker    *CircuitBreaker
}

// NewEvalAgentClient builds the client. endpoint is the eval agent base URL
// (from agents.yaml eval.endpoint). breaker may be nil (no protection).
func NewEvalAgentClient(endpoint string, breaker *CircuitBreaker) *EvalAgentClient {
	return &EvalAgentClient{
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: 5 * time.Second}, // FR-E11: 5s timeout
		breaker:    breaker,
	}
}

// EvalInput is the request body posted to the eval agent's /invoke.
type EvalInput struct {
	DeliverableHash string `json:"deliverable_hash"`
	JobID           string `json:"job_id"`
	JobMetadata     string `json:"job_metadata"`
}

// EvalOutput is the response shape returned by the eval agent.
type EvalOutput struct {
	Score  uint64 `json:"score"` // [0, 1e18]
	Reason string `json:"reason"`
}

// FallbackScore is the default score used when the LLM is unavailable (FR-E11).
// 0.6e18 = 0.6 in fixed-point.
const FallbackScore uint64 = 600_000_000_000_000_000

// Score calls the eval agent and returns (score, reason, decision).
//
// decision is one of: DecisionComplete (LLM returned a valid score),
// DecisionFallback (LLM unavailable, used FallbackScore).
// error is non-nil only on context cancellation.
func (c *EvalAgentClient) Score(ctx context.Context, in EvalInput) (score uint64, reason, decision string, err error) {
	var out EvalOutput
	var callErr error

	if c.breaker != nil {
		callErr = c.breaker.WithBreaker(ctx, func(ctx context.Context) error {
			out, callErr = c.call(ctx, in)
			return callErr
		})
	} else {
		out, callErr = c.call(ctx, in)
	}

	if callErr != nil {
		// Fallback per FR-E11. Don't surface the error to the caller; the
		// Evaluator proceeds with the fallback score and records it.
		return FallbackScore, fmt.Sprintf("fallback: %v", callErr), DecisionFallback, nil
	}

	if out.Score > 1e18 {
		out.Score = 1e18
	}
	if out.Reason == "" {
		out.Reason = "eval agent returned empty reason"
	}
	return out.Score, out.Reason, DecisionComplete, nil
}

// call does the actual HTTP POST to /invoke and parses the response.
func (c *EvalAgentClient) call(ctx context.Context, in EvalInput) (EvalOutput, error) {
	body, err := json.Marshal(struct {
		Input  string `json:"input"`
		Caller string `json:"caller"`
	}{
		Input:  mustJSON(in),
		Caller: "0x0000000000000000000000000000000000000000", // Evaluator identity is logged elsewhere
	})
	if err != nil {
		return EvalOutput{}, fmt.Errorf("marshal eval input: %w", err)
	}

	url := c.endpoint + "/invoke"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return EvalOutput{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return EvalOutput{}, fmt.Errorf("eval agent request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return EvalOutput{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return EvalOutput{}, fmt.Errorf("eval agent returned %d: %s", resp.StatusCode, string(raw))
	}

	// The agent returns {"output": "<json-string>", "proof_hash": "0x..."}.
	// The "output" field is itself a JSON string containing {score, reason}.
	var agentResp struct {
		Output    string `json:"output"`
		ProofHash string `json:"proof_hash"`
	}
	if err := json.Unmarshal(raw, &agentResp); err != nil {
		return EvalOutput{}, fmt.Errorf("parse agent response: %w", err)
	}
	if agentResp.Output == "" {
		return EvalOutput{}, fmt.Errorf("agent output is empty")
	}

	var out EvalOutput
	if err := json.Unmarshal([]byte(agentResp.Output), &out); err != nil {
		return EvalOutput{}, fmt.Errorf("parse agent output JSON: %w", err)
	}
	return out, nil
}

// mustJSON is a tiny helper; only fails on impossible inputs.
func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
