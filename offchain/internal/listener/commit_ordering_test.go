package listener

import (
	"testing"

	"github.com/zane/web3-offchain/model"
)

// simulateCommitLoop replicates tryCommitAll's ordering loop using the pure
// nextReadyTask primitive. It drains every ready task in order, recording the
// commit sequence. This is what tryCommitAll does minus the persistence side
// effects, so we can assert the ordering invariant independent of RPC/DB.
func simulateCommitLoop(completed map[uint64]finishedTask, nextExpected uint64) ([]uint64, uint64) {
	var committed []uint64
	for {
		task, advanced, ok := nextReadyTask(completed, nextExpected)
		if !ok {
			break
		}
		committed = append(committed, task.from)
		delete(completed, nextExpected)
		nextExpected = advanced
	}
	return committed, nextExpected
}

func TestNextReadyTask_Empty(t *testing.T) {
	_, _, ok := nextReadyTask(map[uint64]finishedTask{}, 1)
	if ok {
		t.Fatal("expected ok=false on empty map")
	}
}

func TestNextReadyTask_Present(t *testing.T) {
	completed := map[uint64]finishedTask{5: {from: 5, to: 7}}
	task, advanced, ok := nextReadyTask(completed, 5)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if task.from != 5 || task.to != 7 {
		t.Fatalf("task = {from:%d to:%d}, want {5,7}", task.from, task.to)
	}
	if advanced != 8 {
		t.Fatalf("advanced = %d, want 8", advanced)
	}
}

func TestNextReadyTask_Gap(t *testing.T) {
	// nextExpected=5, but only 6-9 completed -> not ready yet
	completed := map[uint64]finishedTask{6: {from: 6, to: 9}}
	_, _, ok := nextReadyTask(completed, 5)
	if ok {
		t.Fatal("expected ok=false when nextExpected task is missing (gap)")
	}
}

// TestCommitLoop_InOrder verifies a contiguous, in-order completion sequence
// commits all tasks and advances nextExpected to the end.
func TestCommitLoop_InOrder(t *testing.T) {
	completed := map[uint64]finishedTask{
		1:  {from: 1, to: 5},
		6:  {from: 6, to: 10},
		11: {from: 11, to: 15},
	}
	committed, next := simulateCommitLoop(completed, 1)

	if len(committed) != 3 {
		t.Fatalf("committed %d tasks, want 3", len(committed))
	}
	want := []uint64{1, 6, 11}
	for i := range want {
		if committed[i] != want[i] {
			t.Errorf("committed[%d] = %d, want %d", i, committed[i], want[i])
		}
	}
	if next != 16 {
		t.Errorf("nextExpected = %d, want 16", next)
	}
	if len(completed) != 0 {
		t.Errorf("completedTasks not drained: %v", completed)
	}
}

// TestCommitLoop_OutOfOrder is the key invariant: when a later block range
// completes BEFORE the earlier one, the commit loop must NOT commit it. It
// should stall at nextExpected until the gap is filled.
func TestCommitLoop_OutOfOrder(t *testing.T) {
	// Workers complete 6-10 and 11-15, but 1-5 is missing.
	completed := map[uint64]finishedTask{
		6:  {from: 6, to: 10},
		11: {from: 11, to: 15},
	}
	committed, next := simulateCommitLoop(completed, 1)

	if len(committed) != 0 {
		t.Fatalf("committed %v, want none (gap at 1-5 not yet ready)", committed)
	}
	if next != 1 {
		t.Errorf("nextExpected = %d, want 1 (must not advance past gap)", next)
	}
	if len(completed) != 2 {
		t.Errorf("out-of-order tasks should remain buffered, got %d", len(completed))
	}

	// Now the missing gap arrives.
	completed[1] = finishedTask{from: 1, to: 5}
	committed, next = simulateCommitLoop(completed, 1)

	if len(committed) != 3 {
		t.Fatalf("after gap filled, committed %v, want [1 6 11]", committed)
	}
	want := []uint64{1, 6, 11}
	for i := range want {
		if committed[i] != want[i] {
			t.Errorf("committed[%d] = %d, want %d", i, committed[i], want[i])
		}
	}
	if next != 16 {
		t.Errorf("nextExpected = %d, want 16", next)
	}
}

// TestCommitLoop_PartialGap verifies the loop drains everything up to a gap
// and stops exactly at the missing task.
func TestCommitLoop_PartialGap(t *testing.T) {
	completed := map[uint64]finishedTask{
		1: {from: 1, to: 5},
		// 6-10 missing
		11: {from: 11, to: 15},
	}
	committed, next := simulateCommitLoop(completed, 1)

	if len(committed) != 1 || committed[0] != 1 {
		t.Fatalf("committed %v, want [1] (gap at 6)", committed)
	}
	if next != 6 {
		t.Errorf("nextExpected = %d, want 6", next)
	}
	if _, ok := completed[11]; !ok {
		t.Error("out-of-order task 11 should remain buffered")
	}
}

// TestCommitLoop_AdjacentRanges verifies ranges that touch (to+1 == next from)
// still chain correctly.
func TestCommitLoop_AdjacentRanges(t *testing.T) {
	completed := map[uint64]finishedTask{
		1: {from: 1, to: 1, events: []*model.ChainEvent{{}}},
		2: {from: 2, to: 4},
		5: {from: 5, to: 5},
	}
	committed, next := simulateCommitLoop(completed, 1)
	if len(committed) != 3 {
		t.Fatalf("committed %v, want 3 ranges", committed)
	}
	if next != 6 {
		t.Errorf("nextExpected = %d, want 6", next)
	}
}
