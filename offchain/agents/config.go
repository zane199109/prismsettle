package agents

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/viper"
)

// AgentsConfig is the unified configuration for all 4 agents, loaded from
// offchain/config/agents.yaml. Each agent has its own port, model and prompt
// tuning; the LLM provider config (base_url + api_key + model) is shared
// under `llm` but can be overridden per agent.
type AgentsConfig struct {
	LLM   LLMConfig          `mapstructure:"llm"`
	DeFi  AgentRuntimeConfig `mapstructure:"defi"`
	Data  AgentRuntimeConfig `mapstructure:"data_labeling"`
	Trans AgentRuntimeConfig `mapstructure:"translation"`
	Eval  EvalAgentConfig    `mapstructure:"eval"`
}

// LLMConfig is the shared LLM provider configuration. API key is read from
// the env var named by APIKeyEnv (e.g. OPENAI_API_KEY) so secrets never land
// in the YAML. BaseURL allows pointing to a third-party / local model.
type LLMConfig struct {
	BaseURL    string `mapstructure:"base_url"`    // e.g. https://api.openai.com/v1
	APIKeyEnv  string `mapstructure:"api_key_env"` // e.g. OPENAI_API_KEY
	Model      string `mapstructure:"model"`       // e.g. gpt-4o-mini
	TimeoutSec int    `mapstructure:"timeout_sec"` // per-request timeout
}

// AgentRuntimeConfig is the per-agent runtime config shared by the 3 business
// agents. The eval agent has its own struct because it needs extra scoring
// knobs.
type AgentRuntimeConfig struct {
	Port         int    `mapstructure:"port"`
	Endpoint     string `mapstructure:"endpoint"` // public base URL
	Model        string `mapstructure:"model"`    // optional override of LLM.Model
	SystemPrompt string `mapstructure:"system_prompt"`
}

// EvalAgentConfig extends AgentRuntimeConfig with evaluation-specific knobs.
type EvalAgentConfig struct {
	Port         int    `mapstructure:"port"`
	Endpoint     string `mapstructure:"endpoint"`
	Model        string `mapstructure:"model"`
	SystemPrompt string `mapstructure:"system_prompt"`
	// RegistryAddress is the PrismSettle Registry contract address. Used by
	// the eval agent to self-register and by the register subcommand.
	RegistryAddress string `mapstructure:"registry_address"`
}

// Port returns the listen port for the named agent.
func (c *AgentsConfig) Port(name string) (int, error) {
	switch name {
	case "defi":
		return c.DeFi.Port, nil
	case "data_labeling":
		return c.Data.Port, nil
	case "translation":
		return c.Trans.Port, nil
	case "eval":
		return c.Eval.Port, nil
	default:
		return 0, fmt.Errorf("unknown agent: %s", name)
	}
}

// configPath is set via LoadConfig; cached so SIGHUP can reload from disk.
var (
	configMu   sync.RWMutex
	configPtr  *AgentsConfig
	configPath string
)

// LoadConfig loads agents.yaml from the given path and caches it for SIGHUP
// reload. Returns a snapshot the caller can use freely.
func LoadConfig(path string) (*AgentsConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read agents config %s: %w", path, err)
	}
	var c AgentsConfig
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("unmarshal agents config: %w", err)
	}
	if err := c.validate(); err != nil {
		return nil, err
	}

	configMu.Lock()
	configPtr = &c
	configPath = path
	configMu.Unlock()
	return &c, nil
}

// ReloadConfig re-reads the cached config file. Called from SIGHUP handler
// to support hot-reload of API keys (env-var sourced) and prompt tuning.
func ReloadConfig() (*AgentsConfig, error) {
	configMu.RLock()
	path := configPath
	configMu.RUnlock()
	if path == "" {
		return nil, fmt.Errorf("config not loaded yet")
	}
	return LoadConfig(path)
}

// CurrentConfig returns a pointer to the cached config. Safe for concurrent
// reads; callers must not mutate the returned struct.
func CurrentConfig() (*AgentsConfig, error) {
	configMu.RLock()
	defer configMu.RUnlock()
	if configPtr == nil {
		return nil, fmt.Errorf("config not loaded")
	}
	return configPtr, nil
}

// validate enforces fail-fast on boot (project convention §3).
func (c *AgentsConfig) validate() error {
	if c.LLM.BaseURL == "" {
		return fmt.Errorf("agents.yaml: llm.base_url is required")
	}
	if c.LLM.APIKeyEnv == "" {
		return fmt.Errorf("agents.yaml: llm.api_key_env is required")
	}
	if c.LLM.Model == "" {
		return fmt.Errorf("agents.yaml: llm.model is required")
	}
	if c.LLM.TimeoutSec <= 0 {
		c.LLM.TimeoutSec = 30
	}
	if os.Getenv(c.LLM.APIKeyEnv) == "" {
		return fmt.Errorf("agents.yaml: env var %s is not set (LLM API key required)", c.LLM.APIKeyEnv)
	}
	for name, cfg := range map[string]struct {
		port     int
		endpoint string
	}{
		"defi":          {c.DeFi.Port, c.DeFi.Endpoint},
		"data_labeling": {c.Data.Port, c.Data.Endpoint},
		"translation":   {c.Trans.Port, c.Trans.Endpoint},
		"eval":          {c.Eval.Port, c.Eval.Endpoint},
	} {
		if cfg.port == 0 {
			return fmt.Errorf("agents.yaml: %s.port is required", name)
		}
		if cfg.endpoint == "" {
			return fmt.Errorf("agents.yaml: %s.endpoint is required", name)
		}
	}
	if c.Eval.RegistryAddress == "" {
		return fmt.Errorf("agents.yaml: eval.registry_address is required (Phase 5.3 registration)")
	}
	return nil
}
