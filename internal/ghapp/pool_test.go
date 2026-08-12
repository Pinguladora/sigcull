package ghapp

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPoolProcessesAllSubmitted(t *testing.T) {
	var seen atomic.Int64
	var wg sync.WaitGroup
	wg.Add(10)
	pool := NewPool(4, 32, func(_ context.Context, _ Event) {
		seen.Add(1)
		wg.Done()
	}, nil)

	for range 10 {
		if !pool.Submit(Event{HeadSHA: "x"}) {
			t.Fatal("Submit should accept within queue capacity")
		}
	}
	wg.Wait()
	if got := seen.Load(); got != 10 {
		t.Fatalf("processed %d events, want 10", got)
	}

	pool.Shutdown(context.Background())
	if pool.Submit(Event{}) {
		t.Fatal("Submit must return false after shutdown")
	}
}

func TestPoolShedsWhenFull(t *testing.T) {
	// One worker blocked on a gate, queue of 1: the third Submit must be refused.
	gate := make(chan struct{})
	pool := NewPool(1, 1, func(_ context.Context, _ Event) { <-gate }, nil)
	defer func() { close(gate); pool.Shutdown(context.Background()) }()

	// First is picked up by the worker (which then blocks on the gate).
	if !pool.Submit(Event{}) {
		t.Fatal("first submit should be accepted")
	}
	// Give the worker a moment to pull the first job off the queue.
	time.Sleep(20 * time.Millisecond)
	// Second fills the queue.
	if !pool.Submit(Event{}) {
		t.Fatal("second submit should fill the queue")
	}
	// Third has nowhere to go and must be shed.
	if pool.Submit(Event{}) {
		t.Fatal("third submit should be refused when full")
	}
}
