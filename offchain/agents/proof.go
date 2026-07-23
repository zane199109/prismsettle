package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// ProofHash computes a deterministic 0x-prefixed bytes32 hash of the given
// output. The hash is content-addressed: identical outputs yield identical
// proof hashes, which lets downstream consumers (Evaluator, Registry) dedupe
// and verify without re-running the agent.
//
// We hash the output (not the input) because the same input can legitimately
// produce different phrasings of the same answer; the proof hash should
// represent the delivered artifact, not the prompt.
func ProofHash(output string) string {
	sum := sha256.Sum256([]byte(output))
	// Take the first 32 bytes to fit a Solidity bytes32.
	return "0x" + hex.EncodeToString(sum[:32])
}

// FormatScore formats an integer score in [0, 1e18] as a decimal string with
// 18 fractional digits, e.g. FormatScore(6e17) -> "0.600000000000000000".
// Used by the eval agent for human-readable logs; on-chain value is the raw
// uint96.
func FormatScore(score uint64) string {
	whole := score / 1e18
	frac := score % 1e18
	return fmt.Sprintf("%d.%018d", whole, frac)
}
