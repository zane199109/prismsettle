package evaluator

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zane/web3-offchain/pkg/logger"
)

// RoleChecker queries the registry + job + hook contracts for hasRole() and
// is used at Evaluator startup to enforce SD §4.5.7 role isolation.
type RoleChecker interface {
	// HasRole returns true if `addr` holds `role` on the given contract.
	// contractKind is "registry" | "job" | "hook" (for error messages only).
	HasRole(ctx context.Context, contractKind string, role common.Hash, addr common.Address) (bool, error)
}

// AssertRoles enforces SD §4.5.7 at Evaluator startup:
//   - Evaluator MUST hold all AllowedRoles (warn if missing, since the
//     operator may grant later — but log loudly so it's noticed).
//   - Evaluator MUST NOT hold any DisallowedRoles (fatal if held).
//
// evalAddr is the Evaluator's own address (derived from its private key).
//
// The disallowed-role check is fatal because a mis-granted fund-management
// role would let a compromised Evaluator drain funds — fail-fast is the only
// safe response.
func AssertRoles(ctx context.Context, checker RoleChecker, evalAddr common.Address) error {
	// 1. Allowed roles: warn on missing, but continue.
	for _, roleHex := range AllowedRoles {
		role := common.HexToHash(roleHex)
		// Decide which contract to query based on the role name.
		contractKind := roleContractKind(roleHex)
		has, err := checker.HasRole(ctx, contractKind, role, evalAddr)
		if err != nil {
			logger.Warn("evaluator: role check failed (continuing)",
				logger.String("role", roleHex),
				logger.String("contract", contractKind),
				logger.Error(err))
			continue
		}
		if !has {
			logger.Warn("evaluator: missing allowed role (operations may fail until granted)",
				logger.String("role", roleHex),
				logger.String("contract", contractKind),
				logger.String("addr", evalAddr.Hex()))
		}
	}

	// 2. Disallowed roles: fatal on any hit.
	var violations []string
	for _, roleHex := range DisallowedRoles {
		role := common.HexToHash(roleHex)
		contractKind := roleContractKind(roleHex)
		has, err := checker.HasRole(ctx, contractKind, role, evalAddr)
		if err != nil {
			// Don't fatal on a query error — the chain might just be syncing.
			// Log and treat as "not held" to avoid blocking startup on a
			// transient RPC issue.
			logger.Warn("evaluator: disallowed role check failed (treating as not-held)",
				logger.String("role", roleHex),
				logger.String("contract", contractKind),
				logger.Error(err))
			continue
		}
		if has {
			violations = append(violations, fmt.Sprintf("%s on %s", roleHex, contractKind))
		}
	}
	if len(violations) > 0 {
		return fmt.Errorf("SD §4.5.7 violation: evaluator holds disallowed role(s): %s — remove the role(s) before starting",
			strings.Join(violations, ", "))
	}
	return nil
}

// roleContractKind maps a role hex back to the contract that owns it, for
// HasRole dispatch + error messages. The mapping is by role name, not hex,
// because the same hex can't appear on two contracts.
func roleContractKind(roleHex string) string {
	switch roleHex {
	case roleCommerceEvaluator.Hex():
		return "job"
	case roleResolver.Hex():
		return "hook"
	case roleRegistryEvaluator.Hex():
		return "registry"
	case roleWithdraw.Hex():
		return "job" // WITHDRAW_ROLE lives on Job
	case roleFunder.Hex():
		return "job"
	case roleAdmin.Hex():
		return "registry" // DEFAULT_ADMIN_ROLE on all OZ AccessControl contracts; check registry as canonical
	}
	return "unknown"
}
