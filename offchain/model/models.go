package model

import (
	"math/big"
	"time"
)

// EventType enumerates event types
type EventType string

const (
	TypeERC20Transfer  EventType = "ERC20_TRANSFER"
	TypeFundMeFunded   EventType = "FUNDME_FUNDED"
	TypeFundMeRefund   EventType = "FUNDME_REFUND"
	TypeFundMeWithdraw EventType = "FUNDME_WITHDRAW"
	TypeNFTTransfer    EventType = "NFT_TRANSFER"

	// PrismSettle Validation Registry events.
	// agentId is stored as a hex string in the `To` field; validator/owner
	// in `From`; score / amount / newScore in `Value`. This keeps the storage
	// schema unchanged while letting the query layer filter by agentId the
	// same way it filters ERC20 transfers by address.
	TypePrismAgentRegistered     EventType = "PRISM_AGENT_REGISTERED"
	TypePrismValidationSubmitted EventType = "PRISM_VALIDATION_SUBMITTED"
	TypePrismAggregated          EventType = "PRISM_AGGREGATED"
	TypePrismStaked              EventType = "PRISM_STAKED"
	TypePrismSlashed             EventType = "PRISM_SLASHED"
	TypePrismUnstakeStarted      EventType = "PRISM_UNSTAKE_STARTED"
	TypePrismUnstakeWithdrawn    EventType = "PRISM_UNSTAKE_WITHDRAWN"

	// PrismSettleJob lifecycle events.
	// jobId is stored as hex in `To`; agentId/buyer/provider in `From`;
	// amount in `Value`. deliverableHash/proofHash/hook fields are stored in
	// TokenAddr (mirror of Contract) — see parser for field mapping.
	TypePrismJobCreated   EventType = "PRISM_JOB_CREATED"
	TypePrismJobFunded    EventType = "PRISM_JOB_FUNDED"
	TypePrismJobAssigned  EventType = "PRISM_JOB_ASSIGNED"
	TypePrismJobSubmitted EventType = "PRISM_JOB_SUBMITTED"
	TypePrismJobRejected  EventType = "PRISM_JOB_REJECTED"
	TypePrismJobCompleted EventType = "PRISM_JOB_COMPLETED"
	TypePrismJobRefunded  EventType = "PRISM_JOB_REFUNDED"

	// ArbitrationHook events.
	TypePrismDisputed                 EventType = "PRISM_DISPUTED"
	TypePrismDisputeResolved          EventType = "PRISM_DISPUTE_RESOLVED"
	TypePrismArbitratorRegistered     EventType = "PRISM_ARBITRATOR_REGISTERED"
	TypePrismArbitratorUnregistered   EventType = "PRISM_ARBITRATOR_UNREGISTERED"
	TypePrismArbitratorSelected       EventType = "PRISM_ARBITRATOR_SELECTED"

	// PrismSettleJob announcement + arbitration execution events.
	TypePrismDisputeResolvedAnnounced EventType = "PRISM_DISPUTE_RESOLVED_ANNOUNCED"
	TypePrismArbitrationExecuted      EventType = "PRISM_ARBITRATION_EXECUTED"
)

// -----------------------------------------------------------------------------
// Demo session (chat-style agent collaboration playback)
// -----------------------------------------------------------------------------

// DemoSession is one scripted-but-live collaboration run: a real job created
// from a user-facing form, then driven to completion by the orchestrator
// DemoSession holds one chat-style collaboration demo run.
type DemoSession struct {
	ID            string `gorm:"primaryKey;column:id"`
	JobID         string `gorm:"column:job_id"`
	Title         string `gorm:"column:title"`
	Description   string `gorm:"column:description"`
	Amount        string `gorm:"column:amount"`
	Token         string `gorm:"column:token"`
	ProviderAgent string `gorm:"column:provider_agent"`
	Scenario      string `gorm:"column:scenario"` // arbitration | direct
	State         string `gorm:"column:state"`
	MaxRejects    int    `gorm:"column:max_rejects"`
	Result        string `gorm:"column:result;type:text"`
	CreatedAt     time.Time
	FinishedAt    *time.Time
}

// TableName maps DemoSession to demo_sessions.
func (DemoSession) TableName() string { return "demo_sessions" }

// DemoMessage is one chat bubble in a demo session: who said what, and the
// on-chain action (tx) that accompanied the message.
type DemoMessage struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	SessionID string    `gorm:"column:session_id"`
	Step      int       `gorm:"column:step"`
	Role      string    `gorm:"column:role"` // buyer/provider/evaluator/system
	Content   string    `gorm:"column:content;type:text"`
	Report    string    `gorm:"column:report;type:text"` // full deliverable body (markdown)
	DeliverableHash string `gorm:"column:deliverable_hash"` // on-chain keccak of the report
	Action    string    `gorm:"column:action"`           // create_and_fund/grab/submit/reject/dispute/resolve/execute
	TxHash    string    `gorm:"column:tx_hash"`
	State     string    `gorm:"column:state"` // job state after this message
	CreatedAt time.Time `gorm:"column:created_at"`
}

// TableName maps DemoMessage to demo_messages.
func (DemoMessage) TableName() string { return "demo_messages" }

// Transfer transfer event struct
type TransferEvent struct {
	ChainName   string `json:"chain_name"` // Added
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"`
	TxHash      string `json:"tx_hash"`
	LogIndex    uint   `json:"log_index"`
	BlockNumber uint64 `json:"block_number"`
	BlockTime   uint64 `json:"block_time"`
	Contract    string `json:"contract"`
}

type TransferValue struct {
	Value *big.Int `json:"Value"`
}

type ChainEvent struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	ChainName string `gorm:"index;notnull;size:32" json:"chain_name"` // Chain name
	ChainType string `gorm:"index;notnull;size:16" json:"chain_type"` // Fixed: evm
	TxType    string `gorm:"index;size:16" json:"tx_type"`            // native / token
	// Core fields
	EventType   EventType `gorm:"index;notnull;size:32" json:"event_type"`                         // Distinguishes event types, customized per contract
	JobID       string    `gorm:"size:96;index" json:"job_id"`                                     // PrismSettle: jobId (0x-hex) when the event is job-scoped (validation, escrow ops); empty otherwise
	Metadata    string    `gorm:"type:text" json:"metadata"`                                       // AgentRegistered raw metadata string (JSON); empty otherwise
	TokenAddr   string    `gorm:"size:96;index" json:"token_address"`                              // Token address (ERC20) or auxiliary hash field (PrismSettle: proofHash / deliverableHash / hook / reasonHash)
	From        string    `gorm:"size:96;index" json:"from"`                                       // Outgoing address
	To          string    `gorm:"size:96;index" json:"to"`                                         // Incoming address
	Value       string    `gorm:"type:numeric(78,0)" json:"value"`                                 // Amount
	Symbol      string    `gorm:"size:96" json:"symbol"`                                           // Token symbol (ERC20) or auxiliary field (PrismSettle: source / buyer address)
	Decimals    uint8     `gorm:"default:18" json:"decimals"`                                      // Decimals (ERC20) or chain native token (native)
	TxHash      string    `gorm:"size:96;uniqueIndex:idx_tx_hash_index,priority:1" json:"tx_hash"` // Composite unique 1
	LogIndex    uint64    `gorm:"uniqueIndex:idx_tx_hash_index,priority:2" json:"log_index"`       // Composite unique 2
	BlockNumber uint64    `gorm:"index" json:"block_number"`                                       // Block number
	BlockTime   uint64    `gorm:"index" json:"block_time"`                                         // Timestamp
	Contract    string    `gorm:"size:96;index" json:"contract"`                                   // Contract address
	Status      uint8     `gorm:"index;default:1" json:"status"`                                   // 1=Success, 0=Failed
	CreatedAt   time.Time `gorm:"type:timestamp;autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"type:timestamp;autoUpdateTime" json:"updated_at"`
}

// GrabAttempt records an agent's job-grab attempt (success or failure) with
// the reason, so operators can see why they lost a job competition.
type GrabAttempt struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ChainName   string    `gorm:"size:32;index" json:"chain_name"`
	AgentID     string    `gorm:"size:96;index" json:"agent_id"` // hex uint256
	JobID       string    `gorm:"size:96;index" json:"job_id"`   // hex uint256
	Success     bool      `gorm:"index" json:"success"`
	Reason      string    `gorm:"size:255" json:"reason"` // human-readable failure reason
	BlockNumber uint64    `json:"block_number"`
	CreatedAt   time.Time `gorm:"type:timestamp;autoCreateTime" json:"created_at"`
}

type ChainBlockState struct {
	ChainName    string    `gorm:"size:32;not null;uniqueIndex:idx_chain_contract,priority:1" json:"chain_name"`    // Chain name
	ContractAddr string    `gorm:"size:96;not null;uniqueIndex:idx_chain_contract,priority:2" json:"contract_addr"` // Contract address
	LastBlock    uint64    `gorm:"notnull" json:"last_block"`                                                       // Latest processed block height
	BlockHash    string    `gorm:"size:96;index" json:"block_hash"`
	UpdatedAt    time.Time `gorm:"type:timestamp;autoUpdateTime " json:"updated_at"` // Updated at
	CreatedAt    time.Time `gorm:"type:timestamp;autoCreateTime " json:"created_at"` // Created at
}

// ReorgEvent records each chain reorg detected by the listener (Phase 9
// task 9.4). The listener writes one row per reorg with the orphaned block
// range + old/new hash, then the /perf/reorg-feed endpoint reads from this
// table to surface real reorg data on the dashboard (replacing the Phase 7
// mock). FR-M07 + FR-I03/I04.
type ReorgEvent struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ChainName      string    `gorm:"size:32;not null;index:idx_reorg_chain_detected,priority:1" json:"chain_name"`
	ContractAddr   string    `gorm:"size:96;not null" json:"contract_addr"`
	FromBlock      uint64    `gorm:"not null" json:"from_block"` // first orphaned block
	ToBlock        uint64    `gorm:"not null" json:"to_block"`   // last orphaned block (dbLastBlock at detection)
	OldHash        string    `gorm:"size:96" json:"old_hash"`    // hash of dbLastBlock before reorg
	NewHash        string    `gorm:"size:96" json:"new_hash"`    // hash of the same block after reorg
	RollbackDepth  int       `gorm:"not null" json:"rollback_depth"`
	DetectedAt     time.Time `gorm:"type:timestamp;not null;index:idx_reorg_chain_detected,priority:2;autoCreateTime" json:"detected_at"`
	RolledBackRows int64     `gorm:"default:0" json:"rolled_back_rows"` // # of chain_events rows deleted
}

type FundMeStats struct {
	TotalRaised string `json:"total_raised" gorm:"column:total_raised"`
	FunderCount int64  `json:"funder_count" gorm:"column:funder_count"`
	Withdrawn   string `json:"withdrawn" gorm:"column:withdrawn"`
}

type TokenInfo struct {
	Symbol   string
	Decimals uint8
}

// FundMeEventVO FundMe event view object
type FundMeEventVO struct {
	TxHash      string `json:"tx_hash"`
	BlockNumber uint64 `json:"block_number"`
	BlockTime   uint64 `json:"block_time"`
	EventType   string `json:"event_type"`
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"` // String format to avoid precision loss
	Symbol      string `json:"symbol"`
	Decimals    uint8  `json:"decimals"`
	Status      uint8  `json:"status"`
}

// FundMeStatsVO FundMe stats view object
type FundMeStatsVO struct {
	TotalRaised string `json:"total_raised"`
	FunderCount int64  `json:"funder_count"`
	Withdrawn   string `json:"withdrawn"`
	UpdatedAt   int64  `json:"updated_at"`
}

// ConvertToFundMeEventVO converts model to VO
func ConvertToFundMeEventVO(e ChainEvent) *FundMeEventVO {
	return &FundMeEventVO{
		TxHash:      e.TxHash,
		BlockNumber: e.BlockNumber,
		BlockTime:   e.BlockTime,
		EventType:   string(e.EventType),
		From:        e.From,
		To:          e.To,
		Value:       e.Value,
		Symbol:      e.Symbol,
		Decimals:    e.Decimals,
		Status:      e.Status,
	}
}

// ERC20TransferVO ERC20 transfer view object
type ERC20TransferVO struct {
	TxHash      string `json:"tx_hash"`
	BlockNumber uint64 `json:"block_number"`
	BlockTime   uint64 `json:"block_time"`
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"`
	Symbol      string `json:"symbol"`
	Decimals    uint8  `json:"decimals"`
	Contract    string `json:"contract"`
}

// BalanceVO balance view object
type BalanceVO struct {
	Address    string `json:"address"`
	ChainName  string `json:"chain_name"`
	TokenAddr  string `json:"token_address"`
	Symbol     string `json:"symbol"`
	Decimals   uint8  `json:"decimals"`
	BalanceRaw string `json:"balance_raw"`
	BalanceFmt string `json:"balance_formatted"`
}

// TokenInfoVO token info view object
type TokenInfoVO struct {
	Address  string `json:"address"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Decimals uint8  `json:"decimals"`
}

// ConvertToERC20TransferVO converts model to VO
func ConvertToERC20TransferVO(e ChainEvent) *ERC20TransferVO {
	return &ERC20TransferVO{
		TxHash:      e.TxHash,
		BlockNumber: e.BlockNumber,
		BlockTime:   e.BlockTime,
		From:        e.From,
		To:          e.To,
		Value:       e.Value,
		Symbol:      e.Symbol,
		Decimals:    e.Decimals,
		Contract:    e.Contract,
	}
}

// PrismSettleEventVO is the API view object for PrismSettle registry events.
// agentId is decoded from the stored `To` hex field.
type PrismSettleEventVO struct {
	ID          uint64 `json:"id"` // DB id — incremental cursor for consumers
	TxHash      string `json:"tx_hash"`
	BlockNumber uint64 `json:"block_number"`
	BlockTime   uint64 `json:"block_time"`
	EventType   string `json:"event_type"`
	AgentID     string `json:"agent_id"`
	Actor       string `json:"actor"` // validator or agent owner
	Value       string `json:"value"` // score / amount / newScore
	Contract    string `json:"contract"`
	JobID       string `json:"job_id"` // job events: hex uint256
	From        string `json:"from"`   // raw operator address
	To          string `json:"to"`     // raw target (jobId / agentId hex)
}

// PrismSettleScoreVO is the API view object for an agent's latest score.
type PrismSettleScoreVO struct {
	AgentID     string `json:"agent_id"`
	Score       string `json:"score"`
	BlockNumber uint64 `json:"block_number"`
	BlockTime   uint64 `json:"block_time"`
}

// ConvertToPrismSettleEventVO maps a stored ChainEvent into the PrismSettle
// API view. The `To` field holds the agentId hex, `From` holds the actor.
func ConvertToPrismSettleEventVO(e ChainEvent) *PrismSettleEventVO {
	return &PrismSettleEventVO{
		ID:          e.ID,
		TxHash:      e.TxHash,
		BlockNumber: e.BlockNumber,
		BlockTime:   e.BlockTime,
		EventType:   string(e.EventType),
		AgentID:     e.To,
		Actor:       e.From,
		Value:       e.Value,
		Contract:    e.Contract,
		JobID:       e.JobID,
		From:        e.From,
		To:          e.To,
	}
}

// AgentRegistryRecord stores the latest state of an agent as emitted by
// AgentRegistered events. Mirrored off-chain so the /agents endpoint can
// return metadata + endpoint URL without re-parsing event logs.
//
// Phase 7 task 7.1: supports GET /api/v1/prismsettle/agents and
// GET /api/v1/prismsettle/agents/:agentId.
type AgentRegistryRecord struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ChainName    string    `gorm:"size:32;not null;index:idx_agent_chain_id,priority:1;uniqueIndex:idx_agent_unique" json:"chain_name"`
	AgentID      string    `gorm:"size:96;not null;index:idx_agent_chain_id,priority:2;uniqueIndex:idx_agent_unique" json:"agent_id"` // hex uint256
	Owner        string    `gorm:"size:96;index" json:"owner"`                                                                        // address hex
	Metadata     string    `gorm:"type:text" json:"metadata"`                                                                         // raw metadata string emitted by AgentRegistered
	Endpoint     string    `gorm:"size:255" json:"endpoint"`                                                                          // extracted from metadata JSON if present
	RegisteredAt uint64    `gorm:"index" json:"registered_at"`                                                                        // block_time of latest AgentRegistered event
	TxHash       string    `gorm:"size:96" json:"tx_hash"`                                                                            // last seen tx
	BlockNumber  uint64    `gorm:"index" json:"block_number"`
	CreatedAt    time.Time `gorm:"type:timestamp;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"type:timestamp;autoUpdateTime" json:"updated_at"`
}

// TableName overrides the default pluralized table name.
func (AgentRegistryRecord) TableName() string { return "agent_registry" }

// TrustThreshold is the per-agent (or default) trust gate configuration used
// by GET /api/v1/prismsettle/trust (FR-A11 / FR-AP11).
//
// Decision rule:
//
//	score >= AllowThreshold  → ALLOW
//	score <  DenyThreshold   → DENY
//	otherwise                → REQUIRE_VALIDATION
//
// agent_id = "" represents the global default; per-agent rows override it.
type TrustThreshold struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	AgentID        string    `gorm:"size:96;uniqueIndex:idx_trust_agent" json:"agent_id"`                           // "" = global default
	AllowThreshold string    `gorm:"type:numeric(78,0);not null;default:800000000000000000" json:"allow_threshold"` // 0.8e18 default
	DenyThreshold  string    `gorm:"type:numeric(78,0);not null;default:300000000000000000" json:"deny_threshold"`  // 0.3e18 default
	UpdatedAt      time.Time `gorm:"type:timestamp;autoUpdateTime" json:"updated_at"`
	CreatedAt      time.Time `gorm:"type:timestamp;autoCreateTime" json:"created_at"`
}

// TableName overrides the default pluralized table name.
func (TrustThreshold) TableName() string { return "trust_thresholds" }

// TrustDecision enumerates the three-state trust check result (FR-A11).
type TrustDecision string

const (
	TrustAllow             TrustDecision = "ALLOW"
	TrustDeny              TrustDecision = "DENY"
	TrustRequireValidation TrustDecision = "REQUIRE_VALIDATION"
)

// AgentVO is the unified view returned by GET /agents and /agents/:id,
// combining registry metadata with the latest aggregated score.
type AgentVO struct {
	AgentID      string `json:"agent_id"`
	Owner        string `json:"owner"`
	Metadata     string `json:"metadata,omitempty"`
	Endpoint     string `json:"endpoint,omitempty"`
	Score        string `json:"score"`
	RegisteredAt uint64 `json:"registered_at"`
	BlockNumber  uint64 `json:"block_number"`
	TaskCount    int64  `json:"task_count"` // completed jobs as provider
}

// TrustResultVO is the API view for GET /trust.
type TrustResultVO struct {
	AgentID  string        `json:"agent_id"`
	Score    string        `json:"score"`
	Decision TrustDecision `json:"decision"`
	Reason   string        `json:"reason,omitempty"`
}

// JobsListVO is the API view for GET /jobs (jobs filtered by state).
type JobsListVO struct {
	TxHash          string `json:"tx_hash"`
	BlockNumber     uint64 `json:"block_number"`
	BlockTime       uint64 `json:"block_time"`
	EventType       string `json:"event_type"`
	JobID           string `json:"job_id"`
	Buyer           string `json:"buyer"`
	Provider        string `json:"provider"`
	Amount          string `json:"amount"`
	DeliverableHash string `json:"deliverable_hash"`
	Status          string `json:"status"`
}

// JobTimelineVO is the API view for GET /jobs/:jobId/timeline.
type JobTimelineVO struct {
	TxHash      string `json:"tx_hash"`
	BlockNumber uint64 `json:"block_number"`
	BlockTime   uint64 `json:"block_time"`
	EventType   string `json:"event_type"`
	Actor       string `json:"actor"`
	Note        string `json:"note"`
}

// HealthStatusVO is the API view for GET /health (FR-A05 + NFR-OBS02).
//
// Phase 9 extends the original {status, sync_lag, reorg_count} with the
// Evaluator and Keeper runtime status fields required by NFR-OBS02.
// All fields are read from in-memory status trackers injected into the
// handler — there is no DB write path for health data.
type HealthStatusVO struct {
	Status         string `json:"status"`          // "ok" | "degraded" | "down"
	SyncLag        int64  `json:"sync_lag"`        // max blocks behind tip across chains
	ReorgCount     int64  `json:"reorg_count"`     // total reorgs handled since boot
	EvaluatorState string `json:"evaluator_state"` // "running" | "paused" | "stopped"
	KeeperLastRun  int64  `json:"keeper_last_run"` // unix seconds of last keeper tick; 0 = never
}

// ShardActivityVO is the API view for GET /shards/activity (FR-A04).
type ShardActivityVO struct {
	ShardID      uint8  `json:"shard_id"`
	Validations  int64  `json:"validations"`
	LastActivity uint64 `json:"last_activity"`
}

// PerfComparisonVO is the API view for GET /perf/v0-v1-comparison.
// Phase 7 returns mock data; Phase 9 replaces with real benchmark results.
type PerfComparisonVO struct {
	V0AbortRate  float64 `json:"v0_abort_rate"`
	V1AbortRate  float64 `json:"v1_abort_rate"`
	V0Throughput float64 `json:"v0_throughput"`
	V1Throughput float64 `json:"v1_throughput"`
	MeetsFR_T06  bool    `json:"meets_fr_t06"` // V1 abort rate < 5%
	Source       string  `json:"source"`       // "mock" or "benchmark"
}

// PerfResult stores V0 vs V1 benchmark results from a single load-test
// run (Phase 9 task 9.6 / FR-T03/T04). One row per run; the API returns
// the latest row. The Go load generator (offchain/cmd/bench) writes here
// after each completed run, or rows can be inserted manually from CLI logs.
type PerfResult struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	RunID        string    `gorm:"size:64;uniqueIndex" json:"run_id"` // uuid or timestamp slug
	Concurrency  int       `gorm:"not null" json:"concurrency"`       // e.g. 500
	V0AbortRate  float64   `gorm:"not null" json:"v0_abort_rate"`     // 0..1
	V1AbortRate  float64   `gorm:"not null" json:"v1_abort_rate"`
	V0Throughput float64   `gorm:"default:0" json:"v0_throughput"` // txs/s
	V1Throughput float64   `gorm:"default:0" json:"v1_throughput"`
	V0TotalTx    int       `gorm:"default:0" json:"v0_total_tx"`
	V1TotalTx    int       `gorm:"default:0" json:"v1_total_tx"`
	V0AbortedTx  int       `gorm:"default:0" json:"v0_aborted_tx"`
	V1AbortedTx  int       `gorm:"default:0" json:"v1_aborted_tx"`
	MeetsFR_T06  bool      `gorm:"not null" json:"meets_fr_t06"` // V1 abort rate < 5%
	Notes        string    `gorm:"type:text" json:"notes"`       // FR-T06 constraints declaration
	CreatedAt    time.Time `gorm:"type:timestamp;autoCreateTime;index" json:"created_at"`
}

// ReorgFeedItemVO is the API view for GET /perf/reorg-feed.
type ReorgFeedItemVO struct {
	BlockNumber uint64 `json:"block_number"`
	OldHash     string `json:"old_hash"`
	NewHash     string `json:"new_hash"`
	DetectedAt  uint64 `json:"detected_at"`
	Rollback    bool   `json:"rollback"`
}
