package ratelimit

import (
	"sync"
	"sync/atomic"
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

// T5 — floor at zero, full recovery: after a long idle the level floors at 0 (never
// negative), so a fresh burst of 10 is allowed again and no more.
func TestLeakyBucket_FloorsAndFullyRecovers(t *testing.T) {
	now := time.Now()
	lb := NewLeakyBucket(10, time.Minute, fixedClock(&now))
	for i := 0; i < 10; i++ {
		lb.Allow()
	}

	now = now.Add(10 * time.Minute) // far longer than needed to drain
	for i := 1; i <= 10; i++ {
		if d := lb.Allow(); !d.Allowed {
			t.Fatalf("post-idle burst %d: want allowed, got rejected", i)
		}
	}
	if d := lb.Allow(); d.Allowed {
		t.Fatal("post-idle 11th: want rejected (level must not overshoot below 0)")
	}
}

// T6 — Retry-After: on rejection the Decision reports how long until one slot frees
// (~6s when the bucket just filled), and that estimate shrinks as time passes.
func TestLeakyBucket_RetryAfter(t *testing.T) {
	now := time.Now()
	lb := NewLeakyBucket(10, time.Minute, fixedClock(&now))
	for i := 0; i < 10; i++ {
		lb.Allow()
	}

	d := lb.Allow()
	if d.Allowed {
		t.Fatal("bucket should be full")
	}
	// One slot = period/capacity = 6s. Allow a small tolerance for float math.
	if d.RetryAfter < 5*time.Second+900*time.Millisecond || d.RetryAfter > 6*time.Second {
		t.Fatalf("RetryAfter: want ~6s, got %v", d.RetryAfter)
	}

	now = now.Add(3 * time.Second) // half-way to a free slot
	d = lb.Allow()
	if d.Allowed {
		t.Fatal("still full at 3s")
	}
	if d.RetryAfter < 2*time.Second+900*time.Millisecond || d.RetryAfter > 3*time.Second {
		t.Fatalf("RetryAfter after 3s: want ~3s, got %v", d.RetryAfter)
	}
}

// T7 — thread-safety: with a frozen clock (no leak), many goroutines hammering the
// limiter must admit no more than capacity in total. Run with -race to detect any
// unsynchronized access to the shared state.
func TestLeakyBucket_ConcurrentNeverExceedsCapacity(t *testing.T) {
	now := time.Now()
	lb := NewLeakyBucket(10, time.Minute, fixedClock(&now))

	var allowed int64
	var wg sync.WaitGroup
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if lb.Allow().Allowed {
					atomic.AddInt64(&allowed, 1)
				}
			}
		}()
	}
	wg.Wait()

	if allowed != 10 {
		t.Fatalf("concurrent admits: want exactly 10 (capacity), got %d", allowed)
	}
}

// T11 — sliding window allows N, rejects N+1: within one window the first 10 calls
// are allowed and the 11th is rejected.
func TestSlidingWindow_AllowsNRejectsNPlus1(t *testing.T) {
	now := time.Now()
	sw := NewSlidingWindowLog(10, time.Minute, fixedClock(&now))

	for i := 1; i <= 10; i++ {
		if d := sw.Allow(); !d.Allowed {
			t.Fatalf("request %d: want allowed, got rejected", i)
		}
	}
	if d := sw.Allow(); d.Allowed {
		t.Fatal("request 11: want rejected, got allowed")
	}
}

// T12 — oldest ages out: once the window fully passes, the earlier requests leave
// it and capacity is available again.
func TestSlidingWindow_OldestAgesOut(t *testing.T) {
	now := time.Now()
	sw := NewSlidingWindowLog(10, time.Minute, fixedClock(&now))
	for i := 0; i < 10; i++ {
		sw.Allow()
	}
	if d := sw.Allow(); d.Allowed {
		t.Fatal("window should be full")
	}

	now = now.Add(time.Minute + time.Millisecond) // all 10 age out
	for i := 1; i <= 10; i++ {
		if d := sw.Allow(); !d.Allowed {
			t.Fatalf("after window elapsed, request %d: want allowed, got rejected", i)
		}
	}
}

// T13 — strict rolling window: never more than `limit` in ANY 60s span. Ten
// requests are spaced one second apart; a request just before the earliest closes
// out is still rejected, and only once that earliest ages out does exactly one slot
// free. This is the guarantee a leaky bucket cannot make (it allows a boundary
// burst of ~20).
func TestSlidingWindow_StrictRollingWindow(t *testing.T) {
	start := time.Now()
	now := start
	sw := NewSlidingWindowLog(10, time.Minute, fixedClock(&now))

	// 10 requests at start, start+1s, ... start+9s — all within the window.
	for i := 0; i < 10; i++ {
		now = start.Add(time.Duration(i) * time.Second)
		if d := sw.Allow(); !d.Allowed {
			t.Fatalf("spaced request %d: want allowed, got rejected", i+1)
		}
	}

	// At start+59s the earliest (start) is still inside the 60s window → 10 held.
	now = start.Add(59 * time.Second)
	if d := sw.Allow(); d.Allowed {
		t.Fatal("at 59s the window still holds 10: want rejected")
	}

	// At start+60.001s only the earliest ages out → exactly one slot frees.
	now = start.Add(60*time.Second + time.Millisecond)
	if d := sw.Allow(); !d.Allowed {
		t.Fatal("after the earliest ages out: want exactly one slot free (allowed)")
	}
	if d := sw.Allow(); d.Allowed {
		t.Fatal("only one should free: want rejected")
	}
}

// T14 — sliding window Retry-After: on rejection, the hint equals the time until
// the oldest in-window request ages out (~60s when the window just filled, shrinking
// as time passes).
func TestSlidingWindow_RetryAfter(t *testing.T) {
	now := time.Now()
	sw := NewSlidingWindowLog(10, time.Minute, fixedClock(&now))
	for i := 0; i < 10; i++ {
		sw.Allow() // all at t0
	}

	d := sw.Allow()
	if d.Allowed {
		t.Fatal("window should be full")
	}
	if d.RetryAfter < 59*time.Second+900*time.Millisecond || d.RetryAfter > time.Minute {
		t.Fatalf("RetryAfter: want ~60s, got %v", d.RetryAfter)
	}

	now = now.Add(20 * time.Second) // oldest is now 20s old → ~40s until it ages out
	d = sw.Allow()
	if d.Allowed {
		t.Fatal("still full at 20s")
	}
	if d.RetryAfter < 39*time.Second+900*time.Millisecond || d.RetryAfter > 40*time.Second+100*time.Millisecond {
		t.Fatalf("RetryAfter after 20s: want ~40s, got %v", d.RetryAfter)
	}
}
