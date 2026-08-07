package evaluator

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/zane/web3-offchain/pkg/logger"
)

// ArbitrationRule implements FR-E09 dispute resolution logic.
//
// Input: the disputed job's deliverableHash and the buyer's reasonHash.
// Output: ruling (1=buyer refund, 2=provider wins) + a human-readable reason.
//
// Rules (FR-E09):
//  1. reasonHash == 0           → ruling=2 (provider wins; buyer didn't justify)
//  2. reasonHash == deliverableHash → ruling=1 (buyer wins; proves non-delivery)
//  3. reasonHash != deliverableHash → ruling=2 (provider wins; reason mismatch)
//
// ruling=0 is never returned (contract reverts on ruling=0).
type ArbitrationRule struct{}

// NewArbitrationRule builds the rule. Stateless, so a single shared instance
// is fine.
func NewArbitrationRule() *ArbitrationRule { return &ArbitrationRule{} }

// Ruling returns the ruling + reason for a disputed job.
func (a *ArbitrationRule) Ruling(deliverableHash, reasonHash string) (ruling uint8, reason string) {
	deliverableHash = normalizeHash(deliverableHash)
	reasonHash = normalizeHash(reasonHash)

	if isZeroHash(reasonHash) {
		return 2, "FR-E09: buyer did not submit reasonHash, provider wins"
	}
	if reasonHash == deliverableHash {
		return 1, "FR-E09: reasonHash matches deliverableHash, buyer wins refund"
	}
	return 2, "FR-E09: reasonHash != deliverableHash, provider wins"
}

// ArbitrationScore computes the score to record in the Registry after a
// dispute is resolved (FR-E13).
//
//	ruling=2 (provider wins): record the eval agent's quality score so the
//	  provider's reputation still reflects deliverable quality. The score
//	  passed in is the eval agent's score from the main-path evaluation.
//	ruling=1 (buyer wins): record a formulaic penalty
//	  max(0.2e18, currentScore × 30%). A floor of 0.2e18 ensures even a
//	  previously-high-score provider gets a meaningful ding; 30% of current
//	  keeps the penalty proportional to reputation.
//
// currentScore is the provider's current aggregated reputation score (uint96
// in [0, 1e18]); pass 0 if unknown.
//
// The 30% multiplier is computed via big.Int to avoid uint64 overflow
// (currentScore up to 1e18 × 30 = 3e19 > uint64 max).
func ArbitrationScore(ruling uint8, evalScore, currentScore uint64) (uint64, string) {
	switch ruling {
	case 2:
		return evalScore, fmt.Sprintf("FR-E13: provider wins, eval quality score %d recorded", evalScore)
	case 1:
		cur := new(big.Int).SetUint64(currentScore)
		penaltyBig := new(big.Int).Mul(cur, big.NewInt(30))
		penaltyBig.Div(penaltyBig, big.NewInt(100))
		penalty := penaltyBig.Uint64()
		floor := uint64(200_000_000_000_000_000) // 0.2e18
		if penalty < floor {
			penalty = floor
		}
		return penalty, fmt.Sprintf("FR-E13: buyer wins, penalty score %d (max(0.2e18, %d*30%%))", penalty, currentScore)
	default:
		// Should never happen — contract reverts on ruling=0.
		return 0, "FR-E13: invalid ruling 0"
	}
}

// normalizeHash lowercases + trims 0x so comparisons are case-insensitive.
// EIP-55 case differences shouldn't affect dispute resolution.
func normalizeHash(h string) string {
	return strings.ToLower(strings.TrimSpace(h))
}

// isZeroHash returns true if the hex string (0x-prefixed) is all zeros.
func isZeroHash(h string) bool {
	h = strings.TrimPrefix(strings.ToLower(h), "0x")
	return strings.TrimLeft(h, "0") == ""
}

// DisputedJob is the input to the arbitration path: everything the Evaluator
// needs from a Disputed event + job lookup.
type DisputedJob struct {
	JobID           string
	Provider        string
	Buyer           string
	DeliverableHash string // bytes32 hex
	ReasonHash      string // bytes32 hex (the buyer's dispute reason)
}

// Arbitrator ties the arbitration rule + on-chain resolver + reputation
// update into a single Decide method called by the Evaluator main loop when
// it sees a Disputed event.
type Arbitrator struct {
	rule     *ArbitrationRule
	hook     HookResolver   // calls resolveDispute on ArbitrationHook
	registry RegistryWriter // calls submitValidation with source=2
	logs     DecisionStore
}

// HookResolver is the on-chain interface for resolveDispute.
type HookResolver interface {
	// ResolveDispute calls ArbitrationHook.resolveDispute(jobId, ruling).
	// Returns the tx hash.
	ResolveDispute(ctx context.Context, jobID *big.Int, ruling uint8) (string, error)
}

// RegistryWriter is the on-chain interface for submitValidation and
// setAggregatedScore.
type RegistryWriter interface {
	// SubmitValidation calls Registry.submitValidation(agentId, score, proofHash, jobId, source).
	SubmitValidation(ctx context.Context, agentID *big.Int, score uint64, proofHash string, jobID *big.Int, source uint8) (string, error)
	// SetAggregatedScore calls Registry.setAggregatedScore(agentId, newScore).
	// Used by the arbitration path to set penalty scores directly (FR-E13).
	SetAggregatedScore(ctx context.Context, agentID *big.Int, newScore uint64) (string, error)
}

// NewArbitrator builds an Arbitrator.
func NewArbitrator(rule *ArbitrationRule, hook HookResolver, registry RegistryWriter, logs DecisionStore) *Arbitrator {
	return &Arbitrator{rule: rule, hook: hook, registry: registry, logs: logs}
}

// Decide runs the full arbitration flow for a disputed job:
//  1. Look up the provider's current score (passed in by caller).
//  2. Compute ruling via FR-E09.
//  3. Compute penalty score via FR-E13.
//  4. Call resolveDispute(jobId, ruling) on the Hook.
//  5. Call setAggregatedScore(provider, score) on the Registry (FR-E13).
//  6. Record a decision_log row with source=2.
//
// Idempotency: the Evaluator checks HasDecision(jobId, source=2) before
// calling Decide, so Decide is only called once per dispute.
func (a *Arbitrator) Decide(ctx context.Context, job DisputedJob, providerAgentID *big.Int, evalScore, currentScore uint64) error {
	ruling, ruleReason := a.rule.Ruling(job.DeliverableHash, job.ReasonHash)
	score, scoreReason := ArbitrationScore(ruling, evalScore, currentScore)

	logger.Info("arbitration decision",
		logger.String("job_id", job.JobID),
		logger.String("ruling", fmt.Sprintf("%d", ruling)),
		logger.String("rule_reason", ruleReason),
		logger.String("score", fmt.Sprintf("%d", score)),
		logger.String("score_reason", scoreReason))

	jobIDInt := new(big.Int)
	if _, ok := jobIDInt.SetString(job.JobID, 10); !ok {
		return fmt.Errorf("arbitration: parse job_id %q: not a decimal integer", job.JobID)
	}

	// 1. resolveDispute on Hook.
	txHash, err := a.hook.ResolveDispute(ctx, jobIDInt, ruling)
	if err != nil {
		return fmt.Errorf("arbitration: resolveDispute: %w", err)
	}

	// 2. setAggregatedScore on Registry (FR-E13: arbitration penalty).
	// Uses setAggregatedScore instead of submitValidation because the
	// arbitration result is a penalty applied directly to the provider's
	// aggregated score, not a validation record.
	if _, err := a.registry.SetAggregatedScore(ctx, providerAgentID, score); err != nil {
		return fmt.Errorf("arbitration: setAggregatedScore: %w", err)
	}

	// 3. Record decision log.
	dl := &DecisionLog{
		JobID:      job.JobID,
		Source:     SourceEvaluatorArb,
		Decision:   DecisionDisputeResolved,
		Score:      fmt.Sprintf("%d", score),
		Reason:     ruleReason + "; " + scoreReason,
		TxHash:     txHash,
		CallerRole: RoleResolverHex(),
	}
	if err := a.logs.Insert(ctx, dl); err != nil {
		// Non-fatal: on-chain state already changed. Log and continue.
		logger.Errorf("arbitration: insert decision log failed", logger.Error(err))
	}
	return nil
}

// RoleResolverHex returns the hex hash of RESOLVER_ROLE, used in decision_logs.
func RoleResolverHex() string {
	return roleResolver.Hex()
}
