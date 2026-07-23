package parser

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/zane/web3-offchain/model"
)

// ERC20Parser implements EventParser for ERC20 Transfer event parsing
type ERC20Parser struct{}

// EventTransferSig is the keccak256 hash of ERC20 Transfer event signature
// event Transfer(address indexed from, address indexed to, uint256 value)
var EventTransferSig = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

// Compile-time validation: ensure ERC20Parser implements EventParser interface
var _ EventParser = (*ERC20Parser)(nil)

func init() {
	// Auto-register ERC20 parser into the global parser registry
	RegisterParser("ERC20", &ERC20Parser{})
}

// Name returns the parser name for registry and matching
func (p *ERC20Parser) Name() string {
	return "ERC20"
}

// Match checks if the log matches the ERC20 Transfer event signature
func (p *ERC20Parser) Match(log types.Log) bool {
	// Ensure log has at least one topic (event signature)
	if len(log.Topics) == 0 {
		return false
	}

	// Compare topic[0] with the standard Transfer event hash
	return log.Topics[0] == EventTransferSig
}

// Parse decodes the ERC20 Transfer event log into a ChainEvent model
func (p *ERC20Parser) Parse(log types.Log) (any, error) {
	// Validate log structure for a standard ERC20 Transfer event
	// Topics: [signature, from, to]
	if len(log.Topics) < 3 {
		return nil, fmt.Errorf("invalid Transfer event: expected at least 3 topics, got %d", len(log.Topics))
	}

	// Data field must contain uint256 value (32 bytes)
	if len(log.Data) < 32 {
		return nil, fmt.Errorf("invalid Transfer event: data too short (min 32 bytes)")
	}

	// Build structured ChainEvent
	// Addresses are stored with their original EIP-55 checksum casing so the
	// stored value remains self-validating. Query-side LOWER() normalization
	// (see erc20_queries.go) handles case-insensitive matching against API
	// input that may be all-lowercase.
	event := &model.ChainEvent{
		TxHash:      log.TxHash.Hex(),
		BlockNumber: log.BlockNumber,
		LogIndex:    uint64(log.Index),
		Contract:    log.Address.String(),
		EventType:   model.TypeERC20Transfer,
		TokenAddr:   log.Address.Hex(), // Token address is the contract emitting the event
	}

	// Decode indexed from address (topic 1)
	event.From = common.BytesToAddress(log.Topics[1].Bytes()).Hex()

	// Decode indexed to address (topic 2)
	event.To = common.BytesToAddress(log.Topics[2].Bytes()).Hex()

	// Decode uint256 transfer value from data
	event.Value = new(big.Int).SetBytes(log.Data[:32]).String()

	return event, nil
}
