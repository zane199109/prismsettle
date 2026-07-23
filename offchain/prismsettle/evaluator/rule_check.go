package evaluator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// RuleCheck is the pre-evaluation gate (FR-E02 ~ FR-E05). Before invoking the
// eval Agent, the Evaluator runs 4 formal checks on the Submitted event.
//
// The checks are intentionally cheap and side-effect free. IPFS reachability
// (FR-E04) uses HTTP HEAD with a short timeout; unreachable deliverables
// abort the decision (the Evaluator does NOT reject — it just skips and waits
// for re-evaluation next tick).
//
// All methods are goroutine-safe (RuleCheck holds no mutable state).
type RuleCheck struct {
	ipfsGateway string // e.g. https://ipfs.io/ipfs/
	httpClient  *http.Client
}

// NewRuleCheck builds a RuleCheck pointing at the given IPFS gateway. Empty
// gateway disables the HTTP HEAD check (deliverable_hash is treated as
// present but not verified for reachability — useful for dev/tests).
func NewRuleCheck(ipfsGateway string) *RuleCheck {
	return &RuleCheck{
		ipfsGateway: strings.TrimRight(ipfsGateway, "/"),
		httpClient:  &http.Client{Timeout: 5 * time.Second},
	}
}

// RuleResult is the outcome of a RuleCheck run.
type RuleResult struct {
	OK     bool
	Reason string // human-readable reason when OK=false
	Skip   bool   // true = do not reject, just skip this tick (FR-E04)
}

// Check runs all 4 formal checks against a Submitted event.
//
//	FR-E02: deliverable non-empty
//	FR-E03: submitter == provider (source match)
//	FR-E04: deliverable reachable (HTTP HEAD); unreachable → Skip=true
//	FR-E05: proofHash non-zero + IPFS reachable
func (r *RuleCheck) Check(ctx context.Context, job SubmittedJob) RuleResult {
	// FR-E02: deliverable must be non-empty.
	if job.DeliverableHash == "" || isZeroHash(job.DeliverableHash) {
		return RuleResult{OK: false, Reason: "FR-E02: deliverable_hash is empty"}
	}

	// FR-E03: submitter must be the provider.
	if strings.EqualFold(job.Submitter, job.Provider) == false {
		return RuleResult{
			OK:     false,
			Reason: fmt.Sprintf("FR-E03: submitter %s != provider %s", job.Submitter, job.Provider),
		}
	}

	// FR-E04: deliverable reachable (skip-style, not reject).
	if r.ipfsGateway != "" {
		reachable, err := r.isReachable(ctx, job.DeliverableHash)
		if err != nil || !reachable {
			return RuleResult{
				OK:     false,
				Skip:   true, // do not reject; retry next tick
				Reason: fmt.Sprintf("FR-E04: deliverable unreachable: %v", err),
			}
		}
	}

	// FR-E05: proofHash non-zero + reachable.
	if job.ProofHash == "" || isZeroHash(job.ProofHash) {
		return RuleResult{OK: false, Reason: "FR-E05: proof_hash is zero"}
	}
	if r.ipfsGateway != "" {
		reachable, err := r.isReachable(ctx, job.ProofHash)
		if err != nil || !reachable {
			return RuleResult{
				OK:     false,
				Skip:   true,
				Reason: fmt.Sprintf("FR-E05: proof_hash unreachable: %v", err),
			}
		}
	}

	return RuleResult{OK: true}
}

// isReachable does an HTTP HEAD against ipfsGateway/<hash> and returns true
// if the gateway returns any 2xx/3xx status. A short timeout keeps the
// Evaluator tick responsive.
func (r *RuleCheck) isReachable(ctx context.Context, hash string) (bool, error) {
	if r.ipfsGateway == "" {
		return true, nil
	}
	url := r.ipfsGateway + "/" + strings.TrimPrefix(hash, "0x")
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, err
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400, nil
}

// isZeroHash returns true for "" / "0x" / "0x000...0".
func isZeroHash(h string) bool {
	h = strings.TrimPrefix(h, "0x")
	if h == "" {
		return true
	}
	for _, c := range h {
		if c != '0' {
			return false
		}
	}
	return true
}

// SubmittedJob is the input to RuleCheck: the fields the Evaluator needs from
// a Submitted event, normalized to plain strings. The Evaluator fills this
// from the indexer's event store.
type SubmittedJob struct {
	JobID           string
	Provider        string // expected submitter (job.provider from Job contract)
	Submitter       string // actual msg.sender of Submitted event
	DeliverableHash string // bytes32 hex
	ProofHash       string // bytes32 hex
	JobMetadata     string // arbitrary metadata string for the eval agent
}

// ErrRuleCheck is returned by helpers that wrap RuleCheck.
var ErrRuleCheck = errors.New("rule check failed")
