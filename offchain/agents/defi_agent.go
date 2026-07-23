package agents

// DeFiAgent analyzes DeFi market data for arbitrage opportunities.
//
// Input  : market data (prices, pools, liquidity) as text or JSON string
// Output : JSON {"opportunity": bool, "path": [...], "est_apr": "x%", "rationale": "..."}
type DeFiAgent struct {
	BaseAgent
}

// NewDeFiAgent builds the DeFi analysis agent.
func NewDeFiAgent(llm *LLMClient) *DeFiAgent {
	return &DeFiAgent{
		BaseAgent: NewBaseAgent(
			"defi",
			"Analyzes DeFi market data and identifies arbitrage opportunities.",
			[]string{"defi-analysis", "arbitrage-detection"},
			llm,
		),
	}
}

// Invoke runs the agent.
func (a *DeFiAgent) Invoke(input, caller string) (string, string, error) {
	return a.InvokeLLM(input, caller)
}

// Metadata returns the agent.json descriptor.
func (a *DeFiAgent) Metadata() AgentMetadata {
	cfg, err := CurrentConfig()
	endpoint := ""
	if err == nil {
		endpoint = cfg.DeFi.Endpoint
	}
	return AgentMetadata{
		Name:         a.name,
		Description:  a.description,
		Version:      "1.0.0",
		Endpoint:     endpoint,
		Capabilities: a.capabilities,
	}
}
