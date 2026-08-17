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

// T2 — leak frees a slot: after filling the bucket, advancing the clock by one
// leak interval (period/capacity = 6s) drains exactly one slot, so the next call
// is allowed. Proves leak-then-check ordering.
func TestLeakyBucket_LeakFreesASlot(t *testing.T) {
	now := time.Now()
	lb := NewLeakyBucket(10, time.Minute, fixedClock(&now))

	for i := 0; i < 10; i++ {
		lb.Allow()
	}
	if d := lb.Allow(); d.Allowed {
		t.Fatal("bucket should be full")
	}

	now = now.Add(6 * time.Second) // one leak interval
	if d := lb.Allow(); !d.Allowed {
		t.Fatal("after one leak interval: want allowed, got rejected")
	}
	// Only one slot should have freed: the following call is rejected again.
	if d := lb.Allow(); d.Allowed {
		t.Fatal("only one slot should free per interval; want rejected")
	}
}

// T3 — partial leak is proportional: less than one interval frees nothing;
// reaching one full interval frees exactly one slot. Pins the leak rate & units.
func TestLeakyBucket_PartialLeakIsProportional(t *testing.T) {
	now := time.Now()
	lb := NewLeakyBucket(10, time.Minute, fixedClock(&now))
	for i := 0; i < 10; i++ {
		lb.Allow()
	}

	now = now.Add(5 * time.Second) // < 6s interval → no slot yet
	if d := lb.Allow(); d.Allowed {
		t.Fatal("5s < one interval: want rejected, got allowed")
	}
	now = now.Add(1 * time.Second) // now a full 6s has elapsed → one slot
	if d := lb.Allow(); !d.Allowed {
		t.Fatal("6s total: want allowed, got rejected")
	}
}

// T4 — rejections don't consume capacity: a flood of rejected calls must not push
// recovery further out. After filling, hammering the full bucket then waiting one
// interval still frees exactly one slot.
func TestLeakyBucket_RejectionsDoNotConsume(t *testing.T) {
	now := time.Now()
	lb := NewLeakyBucket(10, time.Minute, fixedClock(&now))
	for i := 0; i < 10; i++ {
		lb.Allow()
	}
	for i := 0; i < 50; i++ {
		if d := lb.Allow(); d.Allowed {
			t.Fatal("rejected calls should stay rejected")
		}
	}

	now = now.Add(6 * time.Second)
	if d := lb.Allow(); !d.Allowed {
		t.Fatal("one interval after a reject flood: want allowed (rejects didn't consume)")
	}
}
