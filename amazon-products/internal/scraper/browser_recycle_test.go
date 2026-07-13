package scraper

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// These tests exercise the bounded-close / force-kill recycle path directly,
// without launching a real Chrome process. They assert that the Browser
// self-heals (resets its "started" state) when a tab's graceful close hangs,
// that the force-kill is idempotent per generation (so concurrent recyclers
// cannot double-kill or race), and that a healthy close does not recycle.

// newTestBrowser returns a started Browser at the given epoch whose allocCancel
// increments the returned counter instead of touching a real process.
func newTestBrowser(epoch uint64, killed *atomic.Int32) *Browser {
	b := &Browser{started: true, epoch: epoch}
	b.allocCancel = func() { killed.Add(1) }
	return b
}

func TestRecycleResetsStateAndKillsOnce(t *testing.T) {
	var killed atomic.Int32
	b := newTestBrowser(1, &killed)

	b.recycle(1)

	if b.started {
		t.Fatalf("recycle should reset started to false")
	}
	if got := killed.Load(); got != 1 {
		t.Fatalf("expected allocCancel called once, got %d", got)
	}

	// A second recycle for the same (now-dead) generation is a no-op.
	b.recycle(1)
	if got := killed.Load(); got != 1 {
		t.Fatalf("second recycle should be a no-op, allocCancel called %d times", got)
	}
}

func TestRecycleStaleGenerationIsNoOp(t *testing.T) {
	var killed atomic.Int32
	b := newTestBrowser(2, &killed)

	// A recycle request for an older generation must not kill the live browser.
	b.recycle(1)

	if !b.started {
		t.Fatalf("recycle for a stale epoch must not reset the live browser")
	}
	if got := killed.Load(); got != 0 {
		t.Fatalf("recycle for a stale epoch must not force-kill, got %d calls", got)
	}
}

func TestRecycleIsIdempotentUnderConcurrency(t *testing.T) {
	var killed atomic.Int32
	b := newTestBrowser(1, &killed)

	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			b.recycle(1)
		}()
	}
	wg.Wait()

	if b.started {
		t.Fatalf("browser should be reset after concurrent recycle")
	}
	if got := killed.Load(); got != 1 {
		t.Fatalf("concurrent recycle must force-kill exactly once, got %d", got)
	}
}

func TestCloseTabBoundedRecyclesWhenCloseHangs(t *testing.T) {
	restore := tabCloseTimeout
	tabCloseTimeout = 20 * time.Millisecond
	defer func() { tabCloseTimeout = restore }()

	var killed atomic.Int32
	b := newTestBrowser(1, &killed)

	block := make(chan struct{})
	defer close(block) // release the leaked goroutine at test end
	neverReturns := func() { <-block }

	start := time.Now()
	b.closeTabBounded(neverReturns, "https://example.test/hang", 1)
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Fatalf("closeTabBounded should return promptly after the timeout, took %s", elapsed)
	}
	if b.started {
		t.Fatalf("a hung tab close should recycle the browser")
	}
	if got := killed.Load(); got != 1 {
		t.Fatalf("a hung tab close should force-kill once, got %d", got)
	}
}

func TestCloseTabBoundedDoesNotRecycleOnFastClose(t *testing.T) {
	restore := tabCloseTimeout
	tabCloseTimeout = 50 * time.Millisecond
	defer func() { tabCloseTimeout = restore }()

	var killed atomic.Int32
	b := newTestBrowser(1, &killed)

	b.closeTabBounded(func() {}, "https://example.test/ok", 1)

	if !b.started {
		t.Fatalf("a fast tab close must not recycle the browser")
	}
	if got := killed.Load(); got != 0 {
		t.Fatalf("a fast tab close must not force-kill, got %d calls", got)
	}
}
