package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// LLMClient talks to an OpenAI-compatible chat completions API. It is shared
// by all 4 agents; per-agent model override is honored at call time.
//
// API key is read fresh from the env var on every call so that SIGHUP-driven
// key rotations take effect without restarting the process.
type LLMClient struct {
	httpClient *http.Client
}

// NewLLMClient builds an LLMClient with the configured timeout.
func NewLLMClient(timeoutSec int) *LLMClient {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &LLMClient{
		httpClient: &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
	}
}

// chatMessage is the minimal subset of the OpenAI chat schema we need.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the request body posted to /v1/chat/completions.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	// Deterministic scoring for the eval agent: low temperature + a fixed
	// seed would be ideal, but seed support varies by provider. We instead
	// rely on temperature=0 + caching at the agent layer.
}

// chatResponse is the minimal response shape we parse.
type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Chat sends a single user message under the given system prompt and returns
// the assistant's reply. modelOverride, if non-empty, takes precedence over
// the configured default model.
func (c *LLMClient) Chat(ctx context.Context, cfg *AgentsConfig, systemPrompt, userMessage, modelOverride string) (string, error) {
	apiKey := os.Getenv(cfg.LLM.APIKeyEnv)
	if apiKey == "" {
		return "", fmt.Errorf("env var %s not set", cfg.LLM.APIKeyEnv)
	}
	model := modelOverride
	if model == "" {
		model = cfg.LLM.Model
	}
	if systemPrompt == "" {
		return "", fmt.Errorf("system prompt is required")
	}

	body := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Temperature: 0,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal chat request: %w", err)
	}

	url := cfg.LLM.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("build chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read chat response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("parse chat response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("chat response had no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
