package kernel

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
)

func TestReplayIsDeterministicAndStaleHandlesFailAtomically(t *testing.T) {
	t.Parallel()
	commands := []Command{
		{Sequence: 1, Kind: Spawn, Entity: Handle{ID: 9, Generation: 1}, Delta: 4},
		{Sequence: 2, Kind: Adjust, Entity: Handle{ID: 9, Generation: 1}, Delta: 3},
		{Sequence: 3, Kind: Spawn, Entity: Handle{ID: 2, Generation: 8}, Delta: -1},
	}
	first, second := NewWorld(), NewWorld()
	for _, command := range commands {
		if err := first.Apply(command); err != nil {
			t.Fatal(err)
		}
		if err := second.Apply(command); err != nil {
			t.Fatal(err)
		}
	}
	if first.Snapshot().Digest != second.Snapshot().Digest {
		t.Fatal("identical replay produced different digest")
	}
	before := first.Snapshot()
	if err := first.Apply(Command{Sequence: 4, Kind: Adjust, Entity: Handle{ID: 9, Generation: 2}, Delta: 100}); err == nil {
		t.Fatal("stale handle was accepted")
	}
	if after := first.Snapshot(); before.Digest != after.Digest {
		t.Fatal("rejected command partially mutated state")
	}
}

func TestAdjustmentOverflowAndCanceledSubmissionFailAtomically(t *testing.T) {
	t.Parallel()
	world := NewWorld()
	handle := Handle{ID: 1, Generation: 1}
	if err := world.Apply(Command{Sequence: 1, Kind: Spawn, Entity: handle, Delta: math.MaxInt64}); err != nil {
		t.Fatal(err)
	}
	before := world.Snapshot()
	if err := world.Apply(Command{Sequence: 2, Kind: Adjust, Entity: handle, Delta: 1}); err == nil {
		t.Fatal("overflowing adjustment was accepted")
	}
	if after := world.Snapshot(); after.Digest != before.Digest {
		t.Fatal("overflowing adjustment partially mutated state")
	}

	queue, err := NewQueue[int](1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := queue.PushContext(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled push = %v", err)
	}
	if queue.Len() != 0 {
		t.Fatal("canceled submission entered queue")
	}
}

func TestRemovedHandleRequiresNewGeneration(t *testing.T) {
	t.Parallel()
	world := NewWorld()
	handle := Handle{ID: 7, Generation: 4}
	if err := world.Apply(Command{Sequence: 1, Kind: Spawn, Entity: handle}); err != nil {
		t.Fatal(err)
	}
	if err := world.Apply(Command{Sequence: 2, Kind: Remove, Entity: handle}); err != nil {
		t.Fatal(err)
	}
	before := world.Snapshot()
	if err := world.Apply(Command{Sequence: 3, Kind: Spawn, Entity: handle}); err == nil {
		t.Fatal("retired generation was reused")
	}
	if after := world.Snapshot(); after.Digest != before.Digest {
		t.Fatal("rejected generation reuse changed state")
	}
	newHandle := Handle{ID: 7, Generation: 5}
	if err := world.Apply(Command{Sequence: 3, Kind: Spawn, Entity: newHandle}); err != nil {
		t.Fatal(err)
	}
	if got := world.Snapshot().KnownHandles; len(got) != 1 || got[0] != newHandle {
		t.Fatalf("known handles = %v", got)
	}
}

func TestSequenceOverflowFailsClosed(t *testing.T) {
	t.Parallel()
	world := NewWorld()
	world.lastSequence = math.MaxUint64
	if err := world.Apply(Command{Sequence: 0, Kind: Spawn, Entity: Handle{ID: 1, Generation: 1}}); err == nil {
		t.Fatal("wrapped sequence was accepted")
	}
}

func TestQueueSaturationCloseAndConcurrentSubmission(t *testing.T) {
	t.Parallel()
	queue, err := NewQueue[int](32)
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for value := range 64 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			err := queue.Push(value)
			if err != nil && !errors.Is(err, ErrQueueFull) {
				t.Errorf("push: %v", err)
			}
		}()
	}
	wait.Wait()
	if queue.Len() != 32 {
		t.Fatalf("queue depth = %d, want 32", queue.Len())
	}
	queue.Close()
	if err := queue.Push(65); !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("closed push = %v", err)
	}
	for index := 0; index < 32; index++ {
		if _, ok := queue.Pop(); !ok {
			t.Fatal("accepted item disappeared")
		}
	}
}

func BenchmarkEmptySnapshot(b *testing.B) {
	world := NewWorld()
	for b.Loop() {
		_ = world.Snapshot()
	}
}
