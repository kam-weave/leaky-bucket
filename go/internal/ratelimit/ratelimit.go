// Package ratelimit provides process-local, in-memory request rate limiters.
//
// A RateLimiter is a pluggable admission-control strategy (leaky bucket,
// sliding-window log, ...). Callers depend on the RateLimiter interface, never a
// concrete algorithm, so the strategy can be swapped via configuration without
// touching the HTTP layer (Open/Closed).
package ratelimit

import "time"

// Clock returns the current time. It is injected so tests can advance time
// deterministically instead of sleeping.
type Clock func() time.Time

// Decision is the result of an admission check. RetryAfter is meaningful only
// when Allowed is false: it estimates how long until the next request would be
// admitted, and is surfaced to clients via the HTTP Retry-After header.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

// RateLimiter decides whether a single request may proceed. Implementations must
// be safe for concurrent use.
type RateLimiter interface {
	Allow() Decision
}
