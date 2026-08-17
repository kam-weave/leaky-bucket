package ratelimit

import (
	"sync"
	"time"
)

// LeakyBucket is the counter form of the leaky-bucket algorithm: a "water level"
// that each admitted request raises by one and that leaks continuously at
// capacity/period. A request is admitted while the level stays within capacity,
// otherwise it is rejected. This allows a burst up to capacity, then a steady
// drip of one slot per (period/capacity).
//
// One shared instance serves all clients (the global limit). All access to the
// mutable state runs under mu, so no field is read or written outside the lock.
type LeakyBucket struct {
	capacity float64
	// leakPerNano is the drain rate in units per nanosecond (capacity/period).
	leakPerNano float64
	clock       Clock

	mu    sync.Mutex
	level float64
	last  time.Time
}

// NewLeakyBucket returns a leaky bucket that admits up to capacity requests per
// period (e.g. 10, time.Minute). The clock is injected for deterministic tests.
func NewLeakyBucket(capacity int, period time.Duration, clock Clock) *LeakyBucket {
	return &LeakyBucket{
		capacity:    float64(capacity),
		leakPerNano: float64(capacity) / float64(period),
		clock:       clock,
		last:        clock(),
	}
}

// Allow admits one request if the bucket has room, else rejects it. The entire
// leak-check-increment sequence is a single critical section.
func (b *LeakyBucket) Allow() Decision {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.clock()
	// Clamp elapsed so a backward clock step can never inflate the level.
	if elapsed := now.Sub(b.last); elapsed > 0 {
		b.level -= float64(elapsed.Nanoseconds()) * b.leakPerNano
		if b.level < 0 {
			b.level = 0
		}
	}
	b.last = now

	if b.level+1 <= b.capacity {
		b.level++
		return Decision{Allowed: true}
	}

	// Rejected: report how long until enough leaks out to admit one request, i.e.
	// until the level drops to capacity-1. deficit units at leakPerNano per ns.
	deficit := b.level - (b.capacity - 1)
	return Decision{
		Allowed:    false,
		RetryAfter: time.Duration(deficit / b.leakPerNano),
	}
}
