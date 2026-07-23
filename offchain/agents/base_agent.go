package agents

import (
	"context"
	"fmt"
	"time"
)

// BaseAgent is the shared implementation for the 3 business agents (defi /
// data_labeling / translation). It owns the LLM client and a name used to
// look up per-agent config. Concrete agents embed it and supply their own
// metadata + Invoke (which usually just calls BaseAgent.InvokeLLM).
type BaseAgent struct {
	name         string
	description  string
	capabilities []string
	llm          *LLMClient
}

// NewBaseAgent builds a BaseAgent with the given identity. The LLM client is
// shared across all agents in the process.
func NewBaseAgent(name, description string, caps []string, llm *LLMClient) BaseAgent {
	return BaseAgent{
		name:         name,
		description:  description,
		capabilities: caps,
		llm:          llm,
	}
}

// Name returns the agent identifier.
func (b *BaseAgent) Name() string { return b.name }

// InvokeLLM is the shared invoke path: load current config, fetch this
// agent's system prompt + model override, call the LLM, return its raw
// output together with a content-addressed proof hash.
//
// On config-reload failure we return an error rather than falling back to a
// stale prompt — fail-fast is the project convention.
func (b *BaseAgent) InvokeLLM(input, caller string) (string, string, error) {
	cfg, err := CurrentConfig()
	if err != nil {
		return "", "", fmt.Errorf("%s: load config: %w", b.name, err)
	}
	prompt, model, err := b.runtime(cfg)
	if err != nil {
		return "", "", err
	}

	timeout := time.Duration(cfg.LLM.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	output, err := b.llm.Chat(ctx, cfg, prompt, input, model)
	if err != nil {
		return "", "", fmt.Errorf("%s: llm chat: %w", b.name, err)
	}
	return output, ProofHash(output), nil
}

// runtime returns (system_prompt, model_override) for this agent from the
// current config. model may be "" to inherit the global llm.model.
func (b *BaseAgent) runtime(cfg *AgentsConfig) (string, string, error) {
	switch b.name {
	case "defi":
		return cfg.DeFi.SystemPrompt, cfg.DeFi.Model, nil
	case "data_labeling":
		return cfg.Data.SystemPrompt, cfg.Data.Model, nil
	case "translation":
		return cfg.Trans.SystemPrompt, cfg.Trans.Model, nil
	default:
		return "", "", fmt.Errorf("base agent has no runtime config for %s", b.name)
	}
}
