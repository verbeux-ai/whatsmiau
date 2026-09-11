package whatsmiau

import (
	"context"
	"testing"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
)

// The pairing flow used to share a single 15s budget between the QR code and the
// PairPhone round-trip, and expired the whole attempt as soon as the second one
// ran late — discarding a QR code that had already been generated. waitForCachedValue
// is what makes the two budgets independent: it reports whatever is cached when
// its own budget ends instead of turning a late value into a failure.

func TestWaitForCachedValueReturnsCachedValueImmediately(t *testing.T) {
	cache := xsync.NewMap[string, string]()
	cache.Store("instance", "qr-code")

	start := time.Now()
	got := waitForCachedValue(context.Background(), cache, "instance", 5*time.Second)

	if got != "qr-code" {
		t.Fatalf("got %q, want %q", got, "qr-code")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("a cached value should not wait for the budget, took %s", elapsed)
	}
}

func TestWaitForCachedValueIgnoresEmptyValues(t *testing.T) {
	cache := xsync.NewMap[string, string]()
	cache.Store("instance", "")

	if got := waitForCachedValue(context.Background(), cache, "instance", 50*time.Millisecond); got != "" {
		t.Fatalf("got %q, want an empty result", got)
	}
}

func TestWaitForCachedValuePicksUpValueStoredWhileWaiting(t *testing.T) {
	cache := xsync.NewMap[string, string]()

	go func() {
		time.Sleep(250 * time.Millisecond)
		cache.Store("instance", "pairing-code")
	}()

	if got := waitForCachedValue(context.Background(), cache, "instance", 5*time.Second); got != "pairing-code" {
		t.Fatalf("got %q, want %q", got, "pairing-code")
	}
}

// Regression guard for the pairing 500: a value stored while waiting must be
// returned when the budget itself expires, instead of turning into a failure.
// The store lands at 10ms with a 100ms budget so the 200ms ticker cannot fire
// first — the timer branch (the one that used to produce the 500) is what
// returns the value.
func TestWaitForCachedValueReturnsValueStoredWhileWaitingOnExpiry(t *testing.T) {
	cache := xsync.NewMap[string, string]()

	go func() {
		time.Sleep(10 * time.Millisecond)
		cache.Store("instance", "late-qr-code")
	}()

	if got := waitForCachedValue(context.Background(), cache, "instance", 100*time.Millisecond); got != "late-qr-code" {
		t.Fatalf("got %q, want %q", got, "late-qr-code")
	}
}

func TestWaitForCachedValueReturnsEmptyWhenBudgetExpires(t *testing.T) {
	cache := xsync.NewMap[string, string]()

	if got := waitForCachedValue(context.Background(), cache, "instance", 50*time.Millisecond); got != "" {
		t.Fatalf("got %q, want an empty result", got)
	}
}

func TestWaitForCachedValueStopsWhenContextIsCancelled(t *testing.T) {
	cache := xsync.NewMap[string, string]()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if got := waitForCachedValue(ctx, cache, "instance", 5*time.Second); got != "" {
		t.Fatalf("got %q, want an empty result", got)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("a cancelled context should not wait for the budget, took %s", elapsed)
	}
}

func TestWaitForCachedValueReturnsCachedValueAfterCancellation(t *testing.T) {
	cache := xsync.NewMap[string, string]()
	cache.Store("instance", "qr-code")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if got := waitForCachedValue(ctx, cache, "instance", 5*time.Second); got != "qr-code" {
		t.Fatalf("got %q, want %q", got, "qr-code")
	}
}
