package demo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMClient generates agent "speech" via DeepSeek. The orchestrator always
// validates/overrides action parameters — the model only decides the message
// text and a suggested action, never raw calldata.
type LLMClient struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

func NewLLMClient(baseURL, apiKey, model string) *LLMClient {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	if model == "" {
		model = "deepseek-v4-flash"
	}
	return &LLMClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

// LLLReply is the structured output the model must produce.
type LLMReply struct {
	Action  string  `json:"action"`  // reject/complete/submit/dispute/... (informational)
	Score   *string `json:"score"`   // "0.9" when complete
	Reason  string  `json:"reason"`  // why (reject/dispute justification)
	Content string  `json:"content"` // the chat bubble text (zh)
	Report  string  `json:"report"`  // full deliverable body (markdown) — provider submit only
}

// Speak asks the model to reply as `role` given the conversation so far.
// The system prompt fixes the role; the user message carries job context.
func (c *LLMClient) Speak(ctx context.Context, role, systemPrompt, contextMsg string) (*LLMReply, error) {
	payload := map[string]interface{}{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": contextMsg},
		},
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0.9,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm http %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("llm: empty reply")
	}
	content := parsed.Choices[0].Message.Content
	// Strip any ```json fences the model may add.
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var reply LLMReply
	if err := json.Unmarshal([]byte(content), &reply); err != nil {
		return nil, fmt.Errorf("llm: bad json %v: %s", err, truncate(content, 160))
	}
	if reply.Content == "" {
		reply.Content = reply.Reason
	}
	return &reply, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
