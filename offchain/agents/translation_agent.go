package agents

// TranslationAgent translates text between languages.
//
// Input  : source text, optionally prefixed with "to:<lang>:" to set target
// Output : translated text only
type TranslationAgent struct {
	BaseAgent
}

// NewTranslationAgent builds the translation agent.
func NewTranslationAgent(llm *LLMClient) *TranslationAgent {
	return &TranslationAgent{
		BaseAgent: NewBaseAgent(
			"translation",
			"Translates text between languages.",
			[]string{"translation", "i18n"},
			llm,
		),
	}
}

// Invoke runs the agent.
func (a *TranslationAgent) Invoke(input, caller string) (string, string, error) {
	return a.InvokeLLM(input, caller)
}

// Metadata returns the agent.json descriptor.
func (a *TranslationAgent) Metadata() AgentMetadata {
	cfg, err := CurrentConfig()
	endpoint := ""
	if err == nil {
		endpoint = cfg.Trans.Endpoint
	}
	return AgentMetadata{
		Name:         a.name,
		Description:  a.description,
		Version:      "1.0.0",
		Endpoint:     endpoint,
		Capabilities: a.capabilities,
	}
}
