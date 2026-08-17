package ratelimit

import (
	"testing"
	"time"
)

// fixedClock returns a controllable clock for deterministic tests (no real sleeps).
// Advance time by reassigning *t.
func fixedClock(t *time.Time) Clock {
	return func() time.Time { return *t }
}

// T1 — allow N, reject N+1: with capacity 10, the first 10 calls are allowed and
// the 11th (at level == capacity) is rejected.
func TestLeakyBucket_AllowsNRejectsNPlus1(t *testing.T) {
	now := time.Now()
	lb := NewLeakyBucket(10, time.Minute, fixedClock(&now))

	for i := 1; i <= 10; i++ {
		if d := lb.Allow(); !d.Allowed {
			t.Fatalf("request %d: want allowed, got rejected", i)
		}
	}
	if d := lb.Allow(); d.Allowed {
		t.Fatal("request 11: want rejected, got allowed")
	}
}
