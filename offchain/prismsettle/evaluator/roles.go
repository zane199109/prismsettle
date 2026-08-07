package evaluator

import (
	"github.com/ethereum/go-ethereum/crypto"
)

// Phase 6 / SD §4.5.7 role constants for the PrismSettle Evaluator.
//
// These mirror the role hashes in PrismSettleRegistry.sol / PrismSettleJob.sol
// / ArbitrationHook.sol. Computed at runtime via keccak256 to guarantee they
// match the contract constants exactly (single source of truth = contract
// source). Used by the Evaluator at startup to assert it holds ONLY the
// allowed roles and NONE of the disallowed (fund-management) roles.
//
// V1/V2 role coverage note (SD §4.5.7):
//   - V1 contracts define 4 roles: COMMERCE_EVALUATOR_ROLE, RESOLVER_ROLE,
//     REGISTRY_EVALUATOR_ROLE, DEFAULT_ADMIN_ROLE. Fund flows are guarded by
//     state machine + caller checks (e.g. claimRefund requires msg.sender ==
//     j.buyer), NOT by a privileged WITHDRAW_ROLE.
//   - WITHDRAW_ROLE / FUNDER_ROLE are listed in DisallowedRoles as V2
//     defensive reservations: they don't exist on V1 contracts, so hasRole()
//     returns false and the check is a no-op today. If V2 introduces either
//     role (e.g. for treasury management or emergency withdrawal when TVL
//     crosses the multi-Evaluator threshold), the Evaluator will
//     automatically refuse to start if granted that role — no offchain code
//     change required. This aligns with the SD §4.5.7 wording "不持资金管理
//     角色 (WITHDRAW_ROLE / FUNDER_ROLE 等)" where the "等" indicates the
//     list is illustrative of a role class, not the literal V1 contract set.
var (
	// Allowed roles (Evaluator must hold all three).
	roleCommerceEvaluator = crypto.Keccak256Hash([]byte("COMMERCE_EVALUATOR_ROLE"))
	roleResolver          = crypto.Keccak256Hash([]byte("RESOLVER_ROLE"))
	roleRegistryEvaluator = crypto.Keccak256Hash([]byte("REGISTRY_EVALUATOR_ROLE"))

	// Disallowed roles (Evaluator must NOT hold any; fail-fast on boot).
	// roleWithdraw / roleFunder are V2 defensive reservations — see the
	// V1/V2 note above. They are no-ops on V1 contracts.
	roleWithdraw = crypto.Keccak256Hash([]byte("WITHDRAW_ROLE"))
	roleFunder   = crypto.Keccak256Hash([]byte("FUNDER_ROLE"))
	roleAdmin    = crypto.Keccak256Hash([]byte("DEFAULT_ADMIN_ROLE")) // OpenZeppelin default
)

// AllowedRoles lists the roles the Evaluator is expected to hold. Checked at
// startup; missing any → warn (continue, since the operator may grant later).
//
// Note: COMMERCE_EVALUATOR_ROLE is intentionally excluded — the Evaluator
// no longer calls complete() (Buyer does). The Evaluator only handles
// arbitration (RESOLVER_ROLE on Hook, REGISTRY_EVALUATOR_ROLE on Registry).
var AllowedRoles = []string{
	roleResolver.Hex(),
	roleRegistryEvaluator.Hex(),
}

// DisallowedRoles lists fund-management / admin roles the Evaluator must NOT
// hold. Holding any of these at startup → Fatal (fail-fast, SD §4.5.7).
var DisallowedRoles = []string{
	roleWithdraw.Hex(),
	roleFunder.Hex(),
	roleAdmin.Hex(),
}

// Source enum mirrors the contract's submitValidation source parameter.
const (
	SourceValidator     uint8 = 0
	SourceEvaluatorMain uint8 = 1 // Job completion path (deprecated, kept for DB compat)
	SourceEvaluatorArb  uint8 = 2 // Arbitration path
)

// Decision values stored in decision_logs.decision.
const (
	DecisionComplete        = "complete"
	DecisionFallback        = "fallback" // LLM unavailable, score=0.6e18
	DecisionDisputeResolved = "dispute_resolved"
)
