package ratelimit

import (
	"fmt"
	"time"
)

// Algorithm names selectable via configuration.
const (
	AlgorithmLeakyBucket      = "leaky_bucket"
	AlgorithmSlidingWindowLog = "sliding_window_log"
)

// Config selects and parameterizes a rate limiter. Limit requests are allowed per
// Period (e.g. 10 per time.Minute). Algorithm defaults to the leaky bucket.
type Config struct {
	Algorithm string
	Limit     int
	Period    time.Duration
}

// New builds the RateLimiter named by cfg.Algorithm. This is the single place that
// knows the concrete types; callers depend only on the RateLimiter interface, so a
// new algorithm is added with one case here and needs no change to the HTTP layer
// (Open/Closed).
func New(cfg Config, clock Clock) (RateLimiter, error) {
	switch cfg.Algorithm {
	case AlgorithmLeakyBucket, "":
		return NewLeakyBucket(cfg.Limit, cfg.Period, clock), nil
	case AlgorithmSlidingWindowLog:
		return NewSlidingWindowLog(cfg.Limit, cfg.Period, clock), nil
	default:
		return nil, fmt.Errorf("ratelimit: unknown algorithm %q", cfg.Algorithm)
	}
}
