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

// PrismSettleParser decodes events emitted by PrismSettleRegistry.sol.
//
// Storage mapping onto model.ChainEvent (no schema migration required):
//   - From        = validator / agent owner (address)
//   - To          = agentId (uint256, hex-encoded)
//   - Value       = score / amount / newScore / decay (decimal string)
//   - TokenAddr   = proofHash (bytes32) for ValidationSubmitted; mirror of
//     Contract for other events (preserves existing query parity)
//   - Symbol      = source (e.g. "0"/"1"/"2") for ValidationSubmitted; empty otherwise
//   - EventType   = one of TypePrism*
//   - Contract    = registry address
type PrismSettleParser struct{}

// Compile-time interface compliance check.
var _ parser.EventParser = (*PrismSettleParser)(nil)

func init() {
	parser.RegisterParser("PrismSettle", &PrismSettleParser{})
}

func (p *PrismSettleParser) Name() string { return "PrismSettle" }

// Event signature hashes (keccak256 of the canonical signature).
// MUST match the Solidity event declarations in PrismSettleRegistry.sol exactly.
var (
	// event AgentRegistered(uint256 indexed agentId, address indexed owner, string metadata);
	EventAgentRegisteredSig = crypto.Keccak256Hash([]byte("AgentRegistered(uint256,address,string)"))

	// event ValidationSubmitted(
	//   uint256 indexed agentId,
	//   uint8 indexed shard,
	//   address indexed validator,
	//   uint96 score,
	//   bytes32 proofHash,
	//   uint64 timestamp,
	//   uint8 source,
	//   uint256 jobId
	// );
	EventValidationSubmittedSig = crypto.Keccak256Hash([]byte(
		"ValidationSubmitted(uint256,uint8,address,uint96,bytes32,uint64,uint8,uint256)",
	))

	// event Aggregated(
	//   uint256 indexed agentId,
	//   uint256 oldScore,
	//   uint256 newScore,
	//   uint256 count,
	//   uint64 taskCount,
	//   uint256 decay
	// );
	EventAggregatedSig = crypto.Keccak256Hash([]byte(
		"Aggregated(uint256,uint256,uint256,uint256,uint64,uint256)",
	))

	// event Staked(address indexed validator, uint256 amount);
	EventStakedSig = crypto.Keccak256Hash([]byte("Staked(address,uint256)"))

	// event Slashed(address indexed validator, uint256 amount, bytes32 evidenceHash);
	EventSlashedSig = crypto.Keccak256Hash([]byte("Slashed(address,uint256,bytes32)"))

	// event UnstakeStarted(address indexed validator, uint256 amount, uint64 unlockAt);
	EventUnstakeStartedSig = crypto.Keccak256Hash([]byte("UnstakeStarted(address,uint256,uint64)"))

	// event UnstakeWithdrawn(address indexed validator, uint256 amount);
	EventUnstakeWithdrawnSig = crypto.Keccak256Hash([]byte("UnstakeWithdrawn(address,uint256)"))
)

func (p *PrismSettleParser) Match(log types.Log) bool {
	if len(log.Topics) == 0 {
		return false
	}
	sig := log.Topics[0].Hex()
	return sig == EventAgentRegisteredSig.Hex() ||
		sig == EventValidationSubmittedSig.Hex() ||
		sig == EventAggregatedSig.Hex() ||
		sig == EventStakedSig.Hex() ||
		sig == EventSlashedSig.Hex() ||
		sig == EventUnstakeStartedSig.Hex() ||
		sig == EventUnstakeWithdrawnSig.Hex()
}

func (p *PrismSettleParser) Parse(log types.Log) (any, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("prismsettle: log has no topics")
	}

	// Common fields. TokenAddr mirrors the contract address by default so the
	// query layer's `LOWER(contract) = ?` filter continues to work uniformly;
	// event-specific branches below override TokenAddr when a bytes32 hash
	// field (proofHash / evidenceHash) is present.
	event := &model.ChainEvent{
		TxHash:      log.TxHash.Hex(),
		BlockNumber: log.BlockNumber,
		LogIndex:    uint64(log.Index),
		Contract:    log.Address.String(),
		TokenAddr:   log.Address.Hex(),
	}

	switch log.Topics[0].Hex() {
	case EventAgentRegisteredSig.Hex():
		// Topics: [sig, agentId, owner]
		// Data:   metadata (string, dynamic — offset + length + bytes)
		if len(log.Topics) < 3 {
			return nil, fmt.Errorf("prismsettle AgentRegistered: want 3 topics, got %d", len(log.Topics))
		}
		event.EventType = model.TypePrismAgentRegistered
		event.To = agentIDFromTopic(log.Topics[1])
		event.From = common.BytesToAddress(log.Topics[2].Bytes()).Hex()
		// Decode the ABI-encoded dynamic string: [offset][length][bytes].
		if len(log.Data) >= 64 {
			offset := new(big.Int).SetBytes(log.Data[:32]).Uint64()
			if uint64(len(log.Data)) >= offset+32 {
				length := new(big.Int).SetBytes(log.Data[offset : offset+32]).Uint64()
				if uint64(len(log.Data)) >= offset+32+length {
					event.Metadata = string(log.Data[offset+32 : offset+32+length])
				}
			}
		}
		event.Value = "0"

	case EventValidationSubmittedSig.Hex():
		// Topics: [sig, agentId, shard, validator]
		// Data:   score (uint96, 32 bytes) | proofHash (bytes32) | timestamp (uint64, 32 bytes) | source (uint8, 32 bytes) | jobId (uint256, 32 bytes)
		// Total data length: 5 * 32 = 160 bytes
		if len(log.Topics) < 4 {
			return nil, fmt.Errorf("prismsettle ValidationSubmitted: want 4 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 160 {
			return nil, fmt.Errorf("prismsettle ValidationSubmitted: data too short (min 160 bytes, got %d)", len(log.Data))
		}
		event.EventType = model.TypePrismValidationSubmitted
		event.To = agentIDFromTopic(log.Topics[1])
		event.From = common.BytesToAddress(log.Topics[3].Bytes()).Hex()
		// score is uint96 — left-aligned in 32-byte slot, value fits in big.Int
		event.Value = new(big.Int).SetBytes(log.Data[:32]).String()
		// proofHash is bytes32 — store hex (0x + 64 chars = 66 chars, fits in TokenAddr size:96)
		event.TokenAddr = common.BytesToHash(log.Data[32:64]).Hex()
		// source is uint8 at offset 96 (after score|proofHash|timestamp)
		event.Symbol = fmt.Sprintf("%d", log.Data[96+31]) // last byte of the 32-byte source slot
		// jobId is the 5th data slot (offset 128) — keep job-scoped events
		// linkable across the job's full lifecycle (fund → validate → resolve).
		event.JobID = fmt.Sprintf("0x%064x", new(big.Int).SetBytes(log.Data[128:160]))

	case EventAggregatedSig.Hex():
		// Topics: [sig, agentId]
		// Data:   oldScore (32) | newScore (32) | count (32) | taskCount (32) | decay (32)
		// Total data length: 5 * 32 = 160 bytes
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle Aggregated: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 160 {
			return nil, fmt.Errorf("prismsettle Aggregated: data too short (min 160 bytes, got %d)", len(log.Data))
		}
		event.EventType = model.TypePrismAggregated
		event.To = agentIDFromTopic(log.Topics[1])
		// newScore is the most-queried field for aggregations
		event.Value = new(big.Int).SetBytes(log.Data[32:64]).String()

	case EventStakedSig.Hex():
		// Topics: [sig, validator]
		// Data:   amount (uint256)
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle Staked: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 32 {
			return nil, fmt.Errorf("prismsettle Staked: data too short (min 32 bytes)")
		}
		event.EventType = model.TypePrismStaked
		event.From = common.BytesToAddress(log.Topics[1].Bytes()).Hex()
		event.Value = new(big.Int).SetBytes(log.Data[:32]).String()

	case EventSlashedSig.Hex():
		// Topics: [sig, validator]
		// Data:   amount (uint256) | evidenceHash (bytes32)
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle Slashed: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 64 {
			return nil, fmt.Errorf("prismsettle Slashed: data too short (min 64 bytes)")
		}
		event.EventType = model.TypePrismSlashed
		event.From = common.BytesToAddress(log.Topics[1].Bytes()).Hex()
		event.Value = new(big.Int).SetBytes(log.Data[:32]).String()
		// evidenceHash is bytes32
		event.TokenAddr = common.BytesToHash(log.Data[32:64]).Hex()

	case EventUnstakeStartedSig.Hex():
		// Topics: [sig, validator]
		// Data:   amount (uint256) | unlockAt (uint64)
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle UnstakeStarted: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 64 {
			return nil, fmt.Errorf("prismsettle UnstakeStarted: data too short (min 64 bytes)")
		}
		event.EventType = model.TypePrismUnstakeStarted
		event.From = common.BytesToAddress(log.Topics[1].Bytes()).Hex()
		event.Value = new(big.Int).SetBytes(log.Data[:32]).String()

	case EventUnstakeWithdrawnSig.Hex():
		// Topics: [sig, validator]
		// Data:   amount (uint256)
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle UnstakeWithdrawn: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 32 {
			return nil, fmt.Errorf("prismsettle UnstakeWithdrawn: data too short (min 32 bytes)")
		}
		event.EventType = model.TypePrismUnstakeWithdrawn
		event.From = common.BytesToAddress(log.Topics[1].Bytes()).Hex()
		event.Value = new(big.Int).SetBytes(log.Data[:32]).String()

	default:
		return nil, fmt.Errorf("prismsettle: unknown event sig %s", log.Topics[0].Hex())
	}

	return event, nil
}

// agentIDFromTopic converts a 32-byte indexed topic into a 0x-prefixed hex
// string. agentId is a uint256, so we keep the full 32 bytes (not truncated
// to 20 like an address). The hex form is stable and case-insensitive
// matching on the query side works the same as for addresses.
func agentIDFromTopic(topic common.Hash) string {
	return topic.Hex()
}
