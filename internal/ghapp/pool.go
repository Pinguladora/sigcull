package ghapp

import (
	"context"
	"log/slog"
	"sync"
)

// Pool bounds how many webhook events are processed concurrently. Webhook
// deliveries are bursty and each event clones a repo and runs verification, so
// unbounded goroutines would exhaust disk and CPU. Submit is non-blocking, so a
// full queue sheds load with a 503 rather than piling up.
type Pool struct {
	jobs   chan Event
	fn     func(context.Context, Event)
	log    *slog.Logger
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
	closed bool
}

// NewPool starts a pool of workers draining a bounded queue.
func NewPool(workers, queue int, fn func(context.Context, Event), log *slog.Logger) *Pool {
	if workers < 1 {
		workers = 1
	}
	if queue < 1 {
		queue = 1
	}
	if log == nil {
		log = slog.Default()
	}
	// baseCtx is passed to each worker (not stored on the struct) so Shutdown can
	// cancel in-flight work once the drain deadline passes.
	baseCtx, cancel := context.WithCancel(context.Background())
	p := &Pool{jobs: make(chan Event, queue), fn: fn, log: log, cancel: cancel}
	for range workers {
		p.wg.Add(1)
		go p.worker(baseCtx)
	}
	return p
}

func (p *Pool) worker(ctx context.Context) {
	defer p.wg.Done()
	for ev := range p.jobs {
		p.fn(ctx, ev)
	}
}

// Submit enqueues an event without blocking. It returns false if the pool is
// full or shutting down, so the caller can respond with a 503.
func (p *Pool) Submit(ev Event) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return false
	}
	select {
	case p.jobs <- ev:
		return true
	default:
		return false
	}
}

// Shutdown stops accepting new events and waits for in-flight and queued work to
// drain, or for ctx to expire.
func (p *Pool) Shutdown(ctx context.Context) {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.jobs)
	}
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		// Drain deadline passed. Cancel in-flight verifications so they unwind
		// instead of running to their own longer per-event timeout.
		p.log.Warn("pool shutdown timed out, cancelling in-flight work")
		p.cancel()
	}
	p.cancel() // release the base context
}
