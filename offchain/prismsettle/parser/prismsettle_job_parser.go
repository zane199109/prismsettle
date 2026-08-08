package parser

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/zane/web3-offchain/internal/parser"
	"github.com/zane/web3-offchain/model"
)

// PrismSettleJobParser decodes events emitted by PrismSettleJob.sol.
//
// Storage mapping onto model.ChainEvent (no schema migration required):
//   - To          = jobId (uint256, hex-encoded)
//   - From        = buyer / provider / agentId (depending on event)
//   - Value       = amount (Funded/Completed/Refunded) or 0 (Created/Assigned/Submitted)
//   - TokenAddr   = deliverableHash (Submitted) / hook address (Created) / contract mirror otherwise
//   - EventType   = one of TypePrismJob*
//   - Contract    = job contract address
//
// proofHash field from Submitted event is stored in TokenAddr (FR-JI04).
// hook field from JobCreated event is stored in TokenAddr (FR-JI05).
type PrismSettleJobParser struct{}

var _ parser.EventParser = (*PrismSettleJobParser)(nil)

func init() {
	parser.RegisterParser("PrismSettleJob", &PrismSettleJobParser{})
}

func (p *PrismSettleJobParser) Name() string { return "PrismSettleJob" }

// Event signature hashes (keccak256 of the canonical signature).
// MUST match the Solidity event declarations in PrismSettleJob.sol exactly.
var (
	// event JobCreated(uint256 indexed agentId, uint256 indexed jobId, address buyer, uint64 deadline, address hook, uint96 minProviderReputation, address paymentToken);
	EventJobCreatedSig = crypto.Keccak256Hash([]byte(
		"JobCreated(uint256,uint256,address,uint64,address,uint96,address)",
	))

	// event Funded(uint256 indexed jobId, address buyer, uint256 amount);
	EventFundedSig = crypto.Keccak256Hash([]byte("Funded(uint256,address,uint256)"))

	// event Assigned(uint256 indexed jobId, address provider);
	EventAssignedSig = crypto.Keccak256Hash([]byte("Assigned(uint256,address)"))

	// event Submitted(uint256 indexed jobId, bytes32 deliverableHash, bytes32 proofHash);
	EventSubmittedSig = crypto.Keccak256Hash([]byte("Submitted(uint256,bytes32,bytes32)"))

	// event Rejected(uint256 indexed jobId, address buyer, bytes32 reasonHash);
	EventRejectedSig = crypto.Keccak256Hash([]byte("Rejected(uint256,address,bytes32)"))

	// event Completed(uint256 indexed jobId, address provider, uint256 amount);
	EventCompletedSig = crypto.Keccak256Hash([]byte("Completed(uint256,address,uint256)"))

	// event Refunded(uint256 indexed jobId, address buyer, uint256 amount);
	EventRefundedSig = crypto.Keccak256Hash([]byte("Refunded(uint256,address,uint256)"))

	// event DisputeResolvedAnnounced(uint256 indexed jobId, uint8 ruling, uint256 resolvedAt, uint256 releaseAt);
	EventDisputeResolvedAnnouncedSig = crypto.Keccak256Hash([]byte(
		"DisputeResolvedAnnounced(uint256,uint8,uint256,uint256)",
	))

	// event ArbitrationExecuted(uint256 indexed jobId, uint8 ruling, uint256 amount);
	EventArbitrationExecutedSig = crypto.Keccak256Hash([]byte("ArbitrationExecuted(uint256,uint8,uint256)"))
)

func (p *PrismSettleJobParser) Match(log types.Log) bool {
	if len(log.Topics) == 0 {
		return false
	}
	sig := log.Topics[0].Hex()
	return sig == EventJobCreatedSig.Hex() ||
		sig == EventFundedSig.Hex() ||
		sig == EventAssignedSig.Hex() ||
		sig == EventSubmittedSig.Hex() ||
		sig == EventRejectedSig.Hex() ||
		sig == EventCompletedSig.Hex() ||
		sig == EventRefundedSig.Hex() ||
		sig == EventDisputeResolvedAnnouncedSig.Hex() ||
		sig == EventArbitrationExecutedSig.Hex()
}

func (p *PrismSettleJobParser) Parse(log types.Log) (any, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("prismsettle_job: log has no topics")
	}

	event := &model.ChainEvent{
		TxHash:      log.TxHash.Hex(),
		BlockNumber: log.BlockNumber,
		LogIndex:    uint64(log.Index),
		Contract:    log.Address.String(),
		TokenAddr:   log.Address.Hex(),
	}

	switch log.Topics[0].Hex() {
	case EventJobCreatedSig.Hex():
		// Topics: [sig, agentId, jobId]
		// Data:   buyer (address) | deadline (uint64) | hook (address) |
		//         minProviderReputation (uint96) | paymentToken (address)
		// Total data length: 5 * 32 = 160 bytes
		if len(log.Topics) < 3 {
			return nil, fmt.Errorf("prismsettle_job JobCreated: want 3 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 160 {
			return nil, fmt.Errorf("prismsettle_job JobCreated: data too short (min 160 bytes, got %d)", len(log.Data))
		}
		event.EventType = model.TypePrismJobCreated
		// From = agentId (the agent this job is created for)
		event.From = agentIDFromTopic(log.Topics[1])
		// To = jobId
		event.To = agentIDFromTopic(log.Topics[2])
		// buyer is the first non-indexed data field
		event.Symbol = common.BytesToAddress(log.Data[:32]).Hex()
		// hook address (3rd data slot) stored in TokenAddr (FR-JI05)
		event.TokenAddr = common.BytesToAddress(log.Data[64:96]).Hex()
		event.Value = "0"

	case EventFundedSig.Hex():
		// Topics: [sig, jobId]
		// Data:   buyer (address, 32 bytes) | amount (uint256, 32 bytes)
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle_job Funded: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 64 {
			return nil, fmt.Errorf("prismsettle_job Funded: data too short (min 64 bytes)")
		}
		event.EventType = model.TypePrismJobFunded
		event.To = agentIDFromTopic(log.Topics[1])
		event.From = common.BytesToAddress(log.Data[:32]).Hex()
		event.Value = new(big.Int).SetBytes(log.Data[32:64]).String()

	case EventAssignedSig.Hex():
		// Topics: [sig, jobId]
		// Data:   provider (address, 32 bytes)
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle_job Assigned: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 32 {
			return nil, fmt.Errorf("prismsettle_job Assigned: data too short (min 32 bytes)")
		}
		event.EventType = model.TypePrismJobAssigned
		event.To = agentIDFromTopic(log.Topics[1])
		event.From = common.BytesToAddress(log.Data[:32]).Hex()
		event.Value = "0"

	case EventSubmittedSig.Hex():
		// Topics: [sig, jobId]
		// Data:   deliverableHash (bytes32) | proofHash (bytes32)
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle_job Submitted: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 64 {
			return nil, fmt.Errorf("prismsettle_job Submitted: data too short (min 64 bytes)")
		}
		event.EventType = model.TypePrismJobSubmitted
		event.To = agentIDFromTopic(log.Topics[1])
		// deliverableHash stored in TokenAddr (primary query target, FR-JI04)
		event.TokenAddr = common.BytesToHash(log.Data[:32]).Hex()
		// proofHash stored in From field (size:96 holds 66-char bytes32 hex, FR-JI04).
		// Submitted has no From address, so reusing this slot avoids schema growth.
		event.From = common.BytesToHash(log.Data[32:64]).Hex()
		event.Value = "0"

	case EventRejectedSig.Hex():
		// Topics: [sig, jobId]
		// Data:   buyer (address, 32 bytes) | reasonHash (bytes32, 32 bytes)
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle_job Rejected: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 64 {
			return nil, fmt.Errorf("prismsettle_job Rejected: data too short (min 64 bytes)")
		}
		event.EventType = model.TypePrismJobRejected
		event.To = agentIDFromTopic(log.Topics[1])
		event.From = common.BytesToAddress(log.Data[:32]).Hex()
		// reasonHash stored in TokenAddr (rework request).
		event.TokenAddr = common.BytesToHash(log.Data[32:64]).Hex()
		event.Value = "0"

	case EventCompletedSig.Hex():
		// Topics: [sig, jobId]
		// Data:   provider (address, 32 bytes) | amount (uint256, 32 bytes)
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle_job Completed: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 64 {
			return nil, fmt.Errorf("prismsettle_job Completed: data too short (min 64 bytes)")
		}
		event.EventType = model.TypePrismJobCompleted
		event.To = agentIDFromTopic(log.Topics[1])
		event.From = common.BytesToAddress(log.Data[:32]).Hex()
		event.Value = new(big.Int).SetBytes(log.Data[32:64]).String()

	case EventRefundedSig.Hex():
		// Topics: [sig, jobId]
		// Data:   buyer (address, 32 bytes) | amount (uint256, 32 bytes)
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle_job Refunded: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 64 {
			return nil, fmt.Errorf("prismsettle_job Refunded: data too short (min 64 bytes)")
		}
		event.EventType = model.TypePrismJobRefunded
		event.To = agentIDFromTopic(log.Topics[1])
		event.From = common.BytesToAddress(log.Data[:32]).Hex()
		event.Value = new(big.Int).SetBytes(log.Data[32:64]).String()

	case EventDisputeResolvedAnnouncedSig.Hex():
		// Topics: [sig, jobId]
		// Data:   ruling (uint8, 32 bytes) | resolvedAt (uint256, 32 bytes) | releaseAt (uint256, 32 bytes)
		// Total data length: 3 * 32 = 96 bytes
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle_job DisputeResolvedAnnounced: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 96 {
			return nil, fmt.Errorf("prismsettle_job DisputeResolvedAnnounced: data too short (min 96 bytes, got %d)", len(log.Data))
		}
		event.EventType = model.TypePrismDisputeResolvedAnnounced
		event.To = agentIDFromTopic(log.Topics[1])
		// ruling is uint8, last byte of the 32-byte slot
		event.Value = fmt.Sprintf("%d", log.Data[31])
		// resolvedAt stored in Symbol
		event.Symbol = new(big.Int).SetBytes(log.Data[32:64]).String()
		// releaseAt stored in TokenAddr
		event.TokenAddr = common.BytesToHash(log.Data[64:96]).Hex()

	case EventArbitrationExecutedSig.Hex():
		// Topics: [sig, jobId]
		// Data:   ruling (uint8, 32 bytes) | amount (uint256, 32 bytes)
		// Total data length: 2 * 32 = 64 bytes
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle_job ArbitrationExecuted: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 64 {
			return nil, fmt.Errorf("prismsettle_job ArbitrationExecuted: data too short (min 64 bytes, got %d)", len(log.Data))
		}
		event.EventType = model.TypePrismArbitrationExecuted
		event.To = agentIDFromTopic(log.Topics[1])
		// ruling is uint8, last byte of the 32-byte slot
		event.Value = fmt.Sprintf("%d", log.Data[31])
		// amount
		event.Symbol = new(big.Int).SetBytes(log.Data[32:64]).String()

	default:
		return nil, fmt.Errorf("prismsettle_job: unknown event sig %s", log.Topics[0].Hex())
	}

	// Every PrismSettleJob event is job-scoped: To carries the jobId.
	event.JobID = event.To

	return event, nil
}