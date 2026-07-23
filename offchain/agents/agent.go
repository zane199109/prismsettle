// Package agents implements the 4 Agent services required by PRD FR-AP05.
//
// Each Agent is a small HTTP server exposing:
//
//	POST /invoke                  -> {input, caller} -> {output, proof_hash}
//	GET  /.well-known/agent.json  -> Agent metadata (FR-AP04)
//
// All 4 Agents share the same HTTP scaffolding (server.go) and LLM client
// (llm_client.go). Agent-specific behavior lives in the concrete Agent
// implementations in this package.
package agents

// Agent is the contract every concrete agent must satisfy.
//
// Invoke runs the agent on the given input and returns the output together
// with a 32-byte proof hash that uniquely identifies the computation. The
// caller field is the agentId of the invoking agent (or zero address for a
// direct user) and may be used for accounting / rate-limiting.
type Agent interface {
	// Name returns the unique agent identifier, e.g. "defi", "eval".
	// Used to look up config and to register the agent with the marketplace.
	Name() string

	// Invoke runs the agent.
	// input  - free-form task description (text or JSON string)
	// caller - invoking agentId or zero address (EIP-55 checksum hex)
	// Returns output text and a 0x-prefixed bytes32 proof hash.
	Invoke(input, caller string) (output, proofHash string, err error)

	// Metadata returns the agent.json descriptor published at
	// /.well-known/agent.json. Must be cheap to call; returned fresh each time
	// so callers can mutate without affecting the agent.
	Metadata() AgentMetadata
}

// AgentMetadata is the public description of an agent. Mirrors the A2A
// /.well-known/agent.json schema (subset relevant to PrismSettle).
type AgentMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	// Endpoint is the public base URL where this agent is reachable, e.g.
	// "http://localhost:8001". Populated from config at startup.
	Endpoint string `json:"endpoint"`
	// Capabilities lists the high-level skills the agent provides.
	Capabilities []string `json:"capabilities"`
}
