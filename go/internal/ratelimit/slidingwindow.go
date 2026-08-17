package ratelimit

import (
	"sync"
	"time"
)

// SlidingWindowLog admits at most `limit` requests in any rolling `window`. It
// keeps the timestamps of admitted requests and, on each call, evicts those older
// than the window before counting. Unlike the leaky bucket it enforces a *strict*
// cap (never more than `limit` in any window) at the cost of O(limit) memory and
// no burst-smoothing.
//
// One shared instance serves all clients (the global limit). All access to the
// mutable state runs under mu.
type SlidingWindowLog struct {
	limit  int
	window time.Duration
	clock  Clock

	mu    sync.Mutex
	times []time.Time // admitted timestamps, oldest first
}

// NewSlidingWindowLog returns a limiter that admits up to limit requests per
// rolling window (e.g. 10, time.Minute). The clock is injected for tests.
func NewSlidingWindowLog(limit int, window time.Duration, clock Clock) *SlidingWindowLog {
	return &SlidingWindowLog{limit: limit, window: window, clock: clock}
}

// Allow admits one request if fewer than limit remain within the window.
func (s *SlidingWindowLog) Allow() Decision {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock()
	cutoff := now.Add(-s.window)

	// Drop timestamps that have aged out of the window (time <= cutoff).
	i := 0
	for i < len(s.times) && !s.times[i].After(cutoff) {
		i++
	}
	s.times = s.times[i:]

	if len(s.times) < s.limit {
		s.times = append(s.times, now)
		return Decision{Allowed: true}
	}

	// Rejected: a slot frees when the oldest in-window request ages out, i.e. after
	// window - (now - oldest). Clamp to [0, window] so a backward clock step (now <
	// oldest) can't report a wait longer than the window itself — parity with the
	// leaky bucket's backward-clock defense.
	retryAfter := s.window - now.Sub(s.times[0])
	if retryAfter < 0 {
		retryAfter = 0
	} else if retryAfter > s.window {
		retryAfter = s.window
	}
	return Decision{Allowed: false, RetryAfter: retryAfter}
}
