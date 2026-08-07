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

// PrismSettleHookParser decodes events emitted by ArbitrationHook.sol.
//
// Storage mapping onto model.ChainEvent (no schema migration required):
//   - To          = jobId (uint256, hex-encoded)
//   - From        = arbitrator address (DisputeResolved, ArbitratorRegistered,
//     ArbitratorUnregistered, ArbitratorSelected) or empty
//   - Value       = ruling (DisputeResolved), feeBps (ArbitratorRegistered),
//     score (ArbitratorSelected), or "0"
//   - TokenAddr   = reasonHash (Disputed) / agentId (ArbitratorRegistered) /
//     contract mirror otherwise
//   - EventType   = TypePrism*
//   - Contract    = hook contract address
type PrismSettleHookParser struct{}

var _ parser.EventParser = (*PrismSettleHookParser)(nil)

func init() {
	parser.RegisterParser("PrismSettleHook", &PrismSettleHookParser{})
}

func (p *PrismSettleHookParser) Name() string { return "PrismSettleHook" }

// Event signature hashes (keccak256 of the canonical signature).
// MUST match the Solidity event declarations in ArbitrationHook.sol exactly.
var (
	// event Disputed(uint256 indexed jobId, bytes32 reasonHash);
	EventDisputedSig = crypto.Keccak256Hash([]byte("Disputed(uint256,bytes32)"))

	// event DisputeResolved(uint256 indexed jobId, uint8 ruling, address indexed arbitrator);
	EventDisputeResolvedSig = crypto.Keccak256Hash([]byte("DisputeResolved(uint256,uint8,address)"))

	// event ArbitratorRegistered(address indexed arbitrator, uint256 agentId, uint256 feeBps, address feeRecipient);
	EventArbitratorRegisteredSig = crypto.Keccak256Hash([]byte("ArbitratorRegistered(address,uint256,uint256,address)"))

	// event ArbitratorUnregistered(address indexed arbitrator);
	EventArbitratorUnregisteredSig = crypto.Keccak256Hash([]byte("ArbitratorUnregistered(address)"))

	// event ArbitratorSelected(uint256 indexed jobId, address indexed arbitrator, uint256 score);
	EventArbitratorSelectedSig = crypto.Keccak256Hash([]byte("ArbitratorSelected(uint256,address,uint256)"))
)

func (p *PrismSettleHookParser) Match(log types.Log) bool {
	if len(log.Topics) == 0 {
		return false
	}
	sig := log.Topics[0].Hex()
	return sig == EventDisputedSig.Hex() ||
		sig == EventDisputeResolvedSig.Hex() ||
		sig == EventArbitratorRegisteredSig.Hex() ||
		sig == EventArbitratorUnregisteredSig.Hex() ||
		sig == EventArbitratorSelectedSig.Hex()
}

func (p *PrismSettleHookParser) Parse(log types.Log) (any, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("prismsettle_hook: log has no topics")
	}

	event := &model.ChainEvent{
		TxHash:      log.TxHash.Hex(),
		BlockNumber: log.BlockNumber,
		LogIndex:    uint64(log.Index),
		Contract:    log.Address.String(),
		TokenAddr:   log.Address.Hex(),
	}

	switch log.Topics[0].Hex() {
	case EventDisputedSig.Hex():
		// Topics: [sig, jobId]
		// Data:   reasonHash (bytes32, 32 bytes)
		// NOTE: reasonHash is NOT indexed in the contract
		// (event Disputed(uint256 indexed jobId, bytes32 reasonHash)),
		// so it arrives in the data section, not as a topic.
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle_hook Disputed: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 32 {
			return nil, fmt.Errorf("prismsettle_hook Disputed: data too short (min 32 bytes, got %d)", len(log.Data))
		}
		event.EventType = model.TypePrismDisputed
		event.To = agentIDFromTopic(log.Topics[1])
		// reasonHash stored in TokenAddr (FR-JI06 仲裁态)
		event.TokenAddr = common.BytesToHash(log.Data[:32]).Hex()
		event.Value = "0"

	case EventDisputeResolvedSig.Hex():
		// Topics: [sig, jobId, arbitrator]
		// Data:   ruling (uint8, 32 bytes)
		if len(log.Topics) < 3 {
			return nil, fmt.Errorf("prismsettle_hook DisputeResolved: want 3 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 32 {
			return nil, fmt.Errorf("prismsettle_hook DisputeResolved: data too short (min 32 bytes)")
		}
		event.EventType = model.TypePrismDisputeResolved
		event.To = agentIDFromTopic(log.Topics[1])
		event.From = common.BytesToAddress(log.Topics[2].Bytes()).Hex()
		// ruling is uint8, last byte of the 32-byte slot
		event.Value = fmt.Sprintf("%d", log.Data[31])

	case EventArbitratorRegisteredSig.Hex():
		// Topics: [sig, arbitrator]
		// Data:   agentId (uint256, 32 bytes) | feeBps (uint256, 32 bytes) | feeRecipient (address, 32 bytes)
		// Total data length: 3 * 32 = 96 bytes
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle_hook ArbitratorRegistered: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 96 {
			return nil, fmt.Errorf("prismsettle_hook ArbitratorRegistered: data too short (min 96 bytes, got %d)", len(log.Data))
		}
		event.EventType = model.TypePrismArbitratorRegistered
		event.From = common.BytesToAddress(log.Topics[1].Bytes()).Hex()
		// agentId
		event.TokenAddr = common.BytesToHash(log.Data[:32]).Hex()
		// feeBps
		event.Value = new(big.Int).SetBytes(log.Data[32:64]).String()
		// feeRecipient
		event.Symbol = common.BytesToAddress(log.Data[64:96]).Hex()

	case EventArbitratorUnregisteredSig.Hex():
		// Topics: [sig, arbitrator]
		// Data:   (empty)
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle_hook ArbitratorUnregistered: want 2 topics, got %d", len(log.Topics))
		}
		event.EventType = model.TypePrismArbitratorUnregistered
		event.From = common.BytesToAddress(log.Topics[1].Bytes()).Hex()
		event.Value = "0"

	case EventArbitratorSelectedSig.Hex():
		// Topics: [sig, jobId, arbitrator]
		// Data:   score (uint256, 32 bytes)
		if len(log.Topics) < 3 {
			return nil, fmt.Errorf("prismsettle_hook ArbitratorSelected: want 3 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 32 {
			return nil, fmt.Errorf("prismsettle_hook ArbitratorSelected: data too short (min 32 bytes)")
		}
		event.EventType = model.TypePrismArbitratorSelected
		event.To = agentIDFromTopic(log.Topics[1])
		event.From = common.BytesToAddress(log.Topics[2].Bytes()).Hex()
		event.Value = new(big.Int).SetBytes(log.Data[:32]).String()

	default:
		return nil, fmt.Errorf("prismsettle_hook: unknown event sig %s", log.Topics[0].Hex())
	}

	// From field is used for arbitrator address events; empty for Disputed.
	return event, nil
}
