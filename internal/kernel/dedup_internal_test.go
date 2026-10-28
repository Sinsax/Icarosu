package kernel

import (
	"testing"
	"time"

	"icarosu/internal/sdk/testkit"
)

func TestMarkSeenDeduplicatesAndExpires(t *testing.T) {
	clock := testkit.NewFakeClock(time.Unix(0, 0))
	k := New(Options{Clock: clock, DedupTTL: time.Minute, DedupMax: 2})
	if !k.markSeen("a") {
		t.Fatal("first event should be accepted")
	}
	if k.markSeen("a") {
		t.Fatal("duplicate should be rejected")
	}
	if !k.markSeen("b") {
		t.Fatal("second event should be accepted")
	}
	clock.Advance(2 * time.Minute)
	if !k.markSeen("a") {
		t.Fatal("expired event should be accepted")
	}
	if !k.markSeen("c") {
		t.Fatal("third event should be accepted after eviction")
	}
	if len(k.seen) > 2 {
		t.Fatalf("dedup cache exceeds max: %d", len(k.seen))
	}
}
