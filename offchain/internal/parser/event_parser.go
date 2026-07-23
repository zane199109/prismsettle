package parser

import (
	"github.com/ethereum/go-ethereum/core/types"
)

// EventParser defines the standard interface for parsing blockchain contract events
// All contract parsers (ERC20, FundMe, etc.) must implement this interface
type EventParser interface {
	// Match checks if the log matches the event signature/topic of this parser
	Match(log types.Log) bool

	// Parse decodes the raw log into a structured business model
	// Returns parsed event data or an error if decoding fails
	Parse(log types.Log) (any, error)
}

// ParserRegistry is the global registry for all registered event parsers
// Key: parser name (e.g., "ERC20", "FundMe")
// Value: corresponding EventParser instance
var ParserRegistry = make(map[string]EventParser)

// RegisterParser registers an event parser into the global registry
// This is usually called in the parser's init() function for auto-registration
func RegisterParser(name string, parser EventParser) {
	ParserRegistry[name] = parser
}

// GetParser retrieves a registered parser by name
// Returns nil if the parser does not exist
func GetParser(contractType string) EventParser {
	if p, ok := ParserRegistry[contractType]; ok {
		return p
	}
	return nil
}
