package agents

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestProofHash_Deterministic verifies the same output yields the same proof.
func TestProofHash_Deterministic(t *testing.T) {
	a := ProofHash("hello")
	b := ProofHash("hello")
	if a != b {
		t.Fatalf("ProofHash not deterministic: %s vs %s", a, b)
	}
	if !strings.HasPrefix(a, "0x") || len(a) != 66 {
		t.Fatalf("ProofHash must be 0x-prefixed bytes32 (66 chars), got %q", a)
	}

	c := ProofHash("world")
	if a == c {
		t.Fatalf("ProofHash should differ for different inputs")
	}
}

// TestFormatScore_Boundaries covers 0, midpoint, and 1e18.
func TestFormatScore_Boundaries(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{0, "0.000000000000000000"},
		{600_000_000_000_000_000, "0.600000000000000000"},
		{1_000_000_000_000_000_000, "1.000000000000000000"},
	}
	for _, c := range cases {
		if got := FormatScore(c.in); got != c.want {
			t.Errorf("FormatScore(%d) = %s, want %s", c.in, got, c.want)
		}
	}
}

// TestParseLLMEvalResponse covers plain JSON, fenced JSON, prose-wrapped
// JSON, and malformed inputs.
func TestParseLLMEvalResponse(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
		want    uint64
	}{
		{
			name: "plain_json",
			raw:  `{"score":700000000000000000,"reason":"good"}`,
			want: 700_000_000_000_000_000,
		},
		{
			name: "fenced_json",
			raw:  "```json\n{\"score\":500000000000000000,\"reason\":\"ok\"}\n```",
			want: 500_000_000_000_000_000,
		},
		{
			name: "prose_wrapped",
			raw:  `Here is the score: {"score":1000000000000000000,"reason":"perfect"} hope this helps`,
			want: 1_000_000_000_000_000_000,
		},
		{
			name:    "no_json",
			raw:     "no json here",
			wantErr: true,
		},
		{
			name:    "missing_reason",
			raw:     `{"score":500000000000000000}`,
			wantErr: true,
		},
		{
			name: "score_above_max_clamped_by_caller",
			raw:  `{"score":2000000000000000000,"reason":"too high"}`,
			want: 2_000_000_000_000_000_000, // parser doesn't clamp; caller does
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			le, err := parseLLMEvalResponse(c.raw)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", le)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if le.Score != c.want {
				t.Errorf("score = %d, want %d", le.Score, c.want)
			}
		})
	}
}

// TestEvalAgent_CacheHit verifies that the same deliverable_hash yields the
// same score on a cache hit (variance = 0, FR-E14).
//
// We pre-fill the cache to avoid needing a live LLM in the unit test. The
// Evaluator integration test in Phase 6 exercises the cache-miss path.
func TestEvalAgent_CacheHit(t *testing.T) {
	// Load a config that's missing API key — we should never reach the LLM
	// because cache hit short-circuits. Skip if config can't be loaded.
	cfg, err := loadTestConfig()
	if err != nil {
		t.Skipf("skipping: %v", err)
	}
	configMu.Lock()
	configPtr = cfg
	configMu.Unlock()

	a := NewEvalAgent(nil) // llm unused on cache hit
	hash := "0x" + strings.Repeat("ab", 32)
	a.cache.Store(hash, cachedEval{
		score:  600_000_000_000_000_000,
		reason: "cache test",
	})

	input, _ := json.Marshal(evalInput{
		DeliverableHash: hash,
		JobID:           "42",
		JobMetadata:     "{}",
	})
	out1, proof1, err := a.Invoke(string(input), "0x0")
	if err != nil {
		t.Fatalf("first invoke: %v", err)
	}
	out2, proof2, err := a.Invoke(string(input), "0x0")
	if err != nil {
		t.Fatalf("second invoke: %v", err)
	}
	if out1 != out2 || proof1 != proof2 {
		t.Fatalf("cache hit not deterministic:\n out1=%s\n out2=%s", out1, out2)
	}

	var eo evalOutput
	if err := json.Unmarshal([]byte(out1), &eo); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if eo.Score != 600_000_000_000_000_000 {
		t.Errorf("score = %d, want 6e17", eo.Score)
	}
}

// TestEvalAgent_InputValidation covers bad input shapes.
func TestEvalAgent_InputValidation(t *testing.T) {
	a := NewEvalAgent(nil)
	cases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"not_json", "hello"},
		{"missing_hash", `{"job_id":"1","job_metadata":""}`},
		{"bad_hash_len", `{"deliverable_hash":"0xdead","job_id":"1","job_metadata":""}`},
		{"no_0x_prefix", `{"deliverable_hash":"${"x".repeat(64)}","job_id":"1","job_metadata":""}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := a.Invoke(c.input, "0x0")
			if err == nil {
				t.Fatalf("expected error for input %q", c.input)
			}
		})
	}
}

// loadTestConfig builds an AgentsConfig suitable for tests without requiring
// agents.yaml or an API key env var. The eval agent cache-hit test uses it.
func loadTestConfig() (*AgentsConfig, error) {
	// We bypass validate() because tests don't have OPENAI_API_KEY set.
	return &AgentsConfig{
		LLM: LLMConfig{
			BaseURL:    "http://test",
			APIKeyEnv:  "OPENAI_API_KEY",
			Model:      "gpt-4o-mini",
			TimeoutSec: 5,
		},
		DeFi:  AgentRuntimeConfig{Port: 8001, Endpoint: "http://localhost:8001"},
		Data:  AgentRuntimeConfig{Port: 8002, Endpoint: "http://localhost:8002"},
		Trans: AgentRuntimeConfig{Port: 8003, Endpoint: "http://localhost:8003"},
		Eval: EvalAgentConfig{
			Port:            8004,
			Endpoint:        "http://localhost:8004",
			Model:           "gpt-4o-mini",
			SystemPrompt:    "test prompt",
			RegistryAddress: "0x0000000000000000000000000000000000000001",
		},
	}, nil
}
