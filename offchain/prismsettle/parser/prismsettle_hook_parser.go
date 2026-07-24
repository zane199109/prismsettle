package parser

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/zane/web3-offchain/internal/parser"
	"github.com/zane/web3-offchain/model"
)

// PrismSettleHookParser decodes events emitted by ArbitrationHook.sol.
//
// Storage mapping onto model.ChainEvent (no schema migration required):
//   - To          = jobId (uint256, hex-encoded)
//   - From        = (empty, no address emitted)
//   - Value       = ruling (uint8, for DisputeResolved) or "0" (for Disputed)
//   - TokenAddr   = reasonHash (bytes32, for Disputed) or contract mirror
//   - EventType   = TypePrismDisputed / TypePrismDisputeResolved
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

	// event DisputeResolved(uint256 indexed jobId, uint8 ruling);
	EventDisputeResolvedSig = crypto.Keccak256Hash([]byte("DisputeResolved(uint256,uint8)"))
)

func (p *PrismSettleHookParser) Match(log types.Log) bool {
	if len(log.Topics) == 0 {
		return false
	}
	sig := log.Topics[0].Hex()
	return sig == EventDisputedSig.Hex() ||
		sig == EventDisputeResolvedSig.Hex()
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
		// Topics: [sig, jobId, reasonHash]
		// reasonHash is indexed (bytes32 fits in a topic slot)
		if len(log.Topics) < 3 {
			return nil, fmt.Errorf("prismsettle_hook Disputed: want 3 topics, got %d", len(log.Topics))
		}
		event.EventType = model.TypePrismDisputed
		event.To = agentIDFromTopic(log.Topics[1])
		// reasonHash stored in TokenAddr (FR-JI06 仲裁态)
		event.TokenAddr = log.Topics[2].Hex()
		event.Value = "0"

	case EventDisputeResolvedSig.Hex():
		// Topics: [sig, jobId]
		// Data:   ruling (uint8, 32 bytes)
		if len(log.Topics) < 2 {
			return nil, fmt.Errorf("prismsettle_hook DisputeResolved: want 2 topics, got %d", len(log.Topics))
		}
		if len(log.Data) < 32 {
			return nil, fmt.Errorf("prismsettle_hook DisputeResolved: data too short (min 32 bytes)")
		}
		event.EventType = model.TypePrismDisputeResolved
		event.To = agentIDFromTopic(log.Topics[1])
		// ruling is uint8, last byte of the 32-byte slot
		event.Value = fmt.Sprintf("%d", log.Data[31])

	default:
		return nil, fmt.Errorf("prismsettle_hook: unknown event sig %s", log.Topics[0].Hex())
	}

	// From field is unused for hook events (no address emitted); left empty.
	return event, nil
}
