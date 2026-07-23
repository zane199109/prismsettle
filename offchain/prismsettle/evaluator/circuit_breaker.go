package evaluator

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CircuitBreaker implements the CLOSED → OPEN → HALF_OPEN state machine
// described in SD §4.5.6. It protects the LLM / external-agent call path:
// when the eval agent is repeatedly unavailable, the breaker opens and the
// Evaluator falls back to the 0.6e18 default score without burning requests.
//
// State transitions:
//
//	CLOSED   → OPEN     : consecutive failures reach threshold (default 10)
//	OPEN     → HALF_OPEN: after cooldown (default 60s) a single probe is allowed
//	HALF_OPEN → CLOSED  : probe succeeds
//	HALF_OPEN → OPEN    : probe fails (reset cooldown)
//
// All methods are goroutine-safe.
type CircuitBreaker struct {
	mu sync.Mutex

	state            BreakerState
	failures         int           // consecutive failures since last success
	threshold        int           // failures before opening
	cooldown         time.Duration // open → half-open wait
	openedAt         time.Time     // when the breaker opened
	halfOpenInflight bool          // SD §4.5.6: only one probe in HALF_OPEN
}

// BreakerState is the breaker state enum.
type BreakerState int

const (
	BreakerClosed   BreakerState = iota // normal operation
	BreakerOpen                         // failing fast
	BreakerHalfOpen                     // one probe allowed
)

// NewCircuitBreaker builds a breaker with the given threshold and cooldown.
// threshold <= 0 → 10. cooldown <= 0 → 60s.
func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 10
	}
	if cooldown <= 0 {
		cooldown = 60 * time.Second
	}
	return &CircuitBreaker{
		state:     BreakerClosed,
		threshold: threshold,
		cooldown:  cooldown,
	}
}

// State returns the current state. Mainly for observability / tests.
func (b *CircuitBreaker) State() BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maybeHalfOpen()
	return b.state
}

// Allow reports whether a request may proceed. When Allow returns true the
// caller MUST later call either Success() or Failure() so the breaker can
// update its state. Returning false means "fail fast, do not call the LLM".
//
// In HALF_OPEN only one probe is allowed at a time (halfOpenInflight guard).
func (b *CircuitBreaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maybeHalfOpen()

	switch b.state {
	case BreakerClosed:
		return true
	case BreakerOpen:
		return false
	case BreakerHalfOpen:
		if b.halfOpenInflight {
			return false
		}
		b.halfOpenInflight = true
		return true
	}
	return false
}

// Success records a successful call. Resets the failure counter and closes
// the breaker if it was HALF_OPEN.
func (b *CircuitBreaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.halfOpenInflight = false
	if b.state == BreakerHalfOpen {
		b.state = BreakerClosed
	}
}

// Failure records a failed call. Increments the failure counter and opens the
// breaker if the threshold is reached. In HALF_OPEN a single failure reopens
// the breaker immediately.
func (b *CircuitBreaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.halfOpenInflight = false
	b.failures++
	if b.state == BreakerHalfOpen {
		b.openLocked()
		return
	}
	if b.failures >= b.threshold {
		b.openLocked()
	}
}

// ErrCircuitOpen is returned by callers that respect Allow() == false.
var ErrCircuitOpen = errors.New("circuit breaker open")

// maybeHalfOpen transitions OPEN → HALF_OPEN if the cooldown has elapsed.
// Caller must hold b.mu.
func (b *CircuitBreaker) maybeHalfOpen() {
	if b.state == BreakerOpen && time.Since(b.openedAt) >= b.cooldown {
		b.state = BreakerHalfOpen
		b.halfOpenInflight = false
	}
}

// openLocked transitions to OPEN and records the time. Caller must hold b.mu.
func (b *CircuitBreaker) openLocked() {
	b.state = BreakerOpen
	b.openedAt = time.Now()
}

// WithBreaker runs fn under the breaker's protection. If the breaker is open,
// returns ErrCircuitOpen immediately without calling fn. On success/failure
// of fn, the breaker is updated accordingly. Context cancellation is propagated.
//
// This is the recommended entry point: it eliminates the Allow/Success/Failure
// dance for callers.
func (b *CircuitBreaker) WithBreaker(ctx context.Context, fn func(context.Context) error) error {
	if !b.Allow() {
		return ErrCircuitOpen
	}
	err := fn(ctx)
	if err != nil {
		b.Failure()
		return err
	}
	b.Success()
	return nil
}
