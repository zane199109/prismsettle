package service

// health.go — runtime status tracker for the /health endpoint (Phase 9 task 9.3).
//
// The health endpoint (NFR-OBS02) must report four runtime dimensions:
//   - sync_lag         : max blocks behind tip across all chains
//   - reorg_count      : total reorgs handled since boot
//   - evaluator_state  : "running" | "paused" | "stopped"
//   - keeper_last_run  : unix seconds of the last keeper tick
//
// These values live in memory — they are not persisted. On restart they
// reset, which is the desired behavior for a liveness probe (we care about
// the current process's behavior, not historical aggregates).
//
// Concurrency: all fields are guarded by a single RWMutex because updates
// are infrequent (Evaluator/Keeper tick once per epoch ~minutes) and reads
// are frequent (health endpoint polled every 15s by Docker). A mutex is
// simpler than atomics and the lock contention is negligible.

import (
	"sync"
	"sync/atomic"
	"time"
)

// HealthTracker records runtime status for the /health endpoint.
// Constructed once at boot and injected into the handler.
type HealthTracker struct {
	// reorgCount uses atomic because it is bumped from the listener's
	// reorg-detection hot path (every block tick) where holding a mutex
	// would add avoidable latency.
	reorgCount atomic.Int64

	mu             sync.RWMutex
	syncLag        int64  // max blocks behind tip across chains
	evaluatorState string // "running" | "paused" | "stopped"
	keeperLastRun  int64  // unix seconds
}

// NewHealthTracker returns a tracker in the initial "booting" state.
// The Evaluator and Keeper update their fields once they start.
func NewHealthTracker() *HealthTracker {
	return &HealthTracker{
		evaluatorState: "stopped",
		keeperLastRun:  0,
	}
}

// IncReorg atomically bumps the reorg counter. Called by the listener
// when detectAndHandleReorg rolls back blocks.
func (h *HealthTracker) IncReorg() {
	if h == nil {
		return
	}
	h.reorgCount.Add(1)
}

// SetSyncLag records the current max blocks-behind-tip across all chains.
// Called by the listener's periodic sync loop (typically every commit_step
// blocks). Negative values are clamped to 0 (chain ahead of RPC = clock skew).
func (h *HealthTracker) SetSyncLag(lag int64) {
	if h == nil {
		return
	}
	if lag < 0 {
		lag = 0
	}
	h.mu.Lock()
	h.syncLag = lag
	h.mu.Unlock()
}

// SetEvaluatorState updates the Evaluator runtime state.
// Valid transitions: stopped → running → paused → running → stopped.
func (h *HealthTracker) SetEvaluatorState(state string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.evaluatorState = state
	h.mu.Unlock()
}

// TouchKeeper records that the Keeper just completed a tick.
// Called by the Keeper's main loop after each epoch aggregation pass.
func (h *HealthTracker) TouchKeeper() {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.keeperLastRun = time.Now().Unix()
	h.mu.Unlock()
}

// Snapshot returns a point-in-time copy of all health fields.
// The handler serializes this into the JSON response.
func (h *HealthTracker) Snapshot() (syncLag, reorgCount, keeperLastRun int64, evaluatorState string) {
	if h == nil {
		return 0, 0, 0, "stopped"
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.syncLag, h.reorgCount.Load(), h.keeperLastRun, h.evaluatorState
}

// HealthSnapshot adapts Snapshot to the router.HealthChecker interface.
// The router calls this method directly via the interface — it does not
// import the service package, only the concrete type is wired in main.go.
func (h *HealthTracker) HealthSnapshot() (syncLag, reorgCount, keeperLastRun int64, evaluatorState string) {
	return h.Snapshot()
}
