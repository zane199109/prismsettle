package keeper

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeRegistry is a mock RegistryAggregator for Keeper tests.
type fakeRegistry struct {
	mu         sync.Mutex
	pending    []string
	inactive   []string
	aggregates []string
	decays     []string
	aggErr     error
	decayErr   error
}

func (f *fakeRegistry) PendingAgents(ctx context.Context, limit int) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Drain: return pending once, then empty (simulates aggregation consuming them).
	p := f.pending
	f.pending = nil
	return p, nil
}
func (f *fakeRegistry) AggregateEpoch(ctx context.Context, agentID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.aggErr != nil {
		return "", f.aggErr
	}
	f.aggregates = append(f.aggregates, agentID)
	return "0xAGG", nil
}
func (f *fakeRegistry) InactiveAgents(ctx context.Context, inactiveAge time.Duration, limit int) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Drain on read so the scan doesn't re-fire on the next tick.
	a := f.inactive
	f.inactive = nil
	return a, nil
}
func (f *fakeRegistry) TickDecay(ctx context.Context, agentID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.decayErr != nil {
		return "", f.decayErr
	}
	f.decays = append(f.decays, agentID)
	return "0xDECAY", nil
}

func TestKeeper_AggregateBatch(t *testing.T) {
	reg := &fakeRegistry{pending: []string{"1", "2", "3"}}
	k, err := NewKeeper(Config{TickPeriod: 50 * time.Millisecond, BatchSize: 10}, reg)
	if err != nil {
		t.Fatalf("NewKeeper: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_ = k.Run(ctx)

	reg.mu.Lock()
	defer reg.mu.Unlock()
	if len(reg.aggregates) != 3 {
		t.Errorf("expected 3 aggregate calls, got %d", len(reg.aggregates))
	}
}

func TestKeeper_InactiveScan(t *testing.T) {
	reg := &fakeRegistry{inactive: []string{"10", "20"}}
	k, err := NewKeeper(Config{
		TickPeriod:  1 * time.Second, // slow tick so only scan fires
		ScanPeriod:  50 * time.Millisecond,
		InactiveAge: 30 * 24 * time.Hour,
		BatchSize:   10,
	}, reg)
	if err != nil {
		t.Fatalf("NewKeeper: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = k.Run(ctx)

	reg.mu.Lock()
	defer reg.mu.Unlock()
	if len(reg.decays) != 2 {
		t.Errorf("expected 2 decay calls, got %d", len(reg.decays))
	}
}

func TestKeeper_AggregateErrorContinues(t *testing.T) {
	reg := &fakeRegistry{
		pending: []string{"1", "2"},
		aggErr:  fmt.Errorf("rpc error"),
	}
	k, err := NewKeeper(Config{TickPeriod: 50 * time.Millisecond}, reg)
	if err != nil {
		t.Fatalf("NewKeeper: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_ = k.Run(ctx)

	// Errors should not crash the keeper; no aggregates recorded but no panic.
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if len(reg.aggregates) != 0 {
		t.Errorf("expected 0 successful aggregates, got %d", len(reg.aggregates))
	}
}

func TestKeeper_NoPendingNoOp(t *testing.T) {
	reg := &fakeRegistry{pending: nil}
	k, err := NewKeeper(Config{TickPeriod: 50 * time.Millisecond}, reg)
	if err != nil {
		t.Fatalf("NewKeeper: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = k.Run(ctx)

	reg.mu.Lock()
	defer reg.mu.Unlock()
	if len(reg.aggregates) != 0 {
		t.Errorf("expected 0 aggregates when no pending, got %d", len(reg.aggregates))
	}
}

func TestNewKeeper_NilRegistry(t *testing.T) {
	_, err := NewKeeper(Config{}, nil)
	if err == nil {
		t.Fatal("expected error on nil registry")
	}
}
