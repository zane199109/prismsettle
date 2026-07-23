package agents

// DataLabelingAgent labels unlabeled data samples.
//
// Input  : sample to label (text, JSON, etc.)
// Output : JSON {"label": "...", "confidence": 0.0-1.0, "rationale": "..."}
type DataLabelingAgent struct {
	BaseAgent
}

// NewDataLabelingAgent builds the data labeling agent.
func NewDataLabelingAgent(llm *LLMClient) *DataLabelingAgent {
	return &DataLabelingAgent{
		BaseAgent: NewBaseAgent(
			"data_labeling",
			"Labels unlabeled data samples with confidence scores.",
			[]string{"classification", "labeling"},
			llm,
		),
	}
}

// Invoke runs the agent.
func (a *DataLabelingAgent) Invoke(input, caller string) (string, string, error) {
	return a.InvokeLLM(input, caller)
}

// Metadata returns the agent.json descriptor.
func (a *DataLabelingAgent) Metadata() AgentMetadata {
	cfg, err := CurrentConfig()
	endpoint := ""
	if err == nil {
		endpoint = cfg.Data.Endpoint
	}
	return AgentMetadata{
		Name:         a.name,
		Description:  a.description,
		Version:      "1.0.0",
		Endpoint:     endpoint,
		Capabilities: a.capabilities,
	}
}
