# Design Tradeoffs & Future Work

Deliberate decisions in the rate limiter, their costs, and how each would evolve. A theme: the
algorithm sits behind the `RateLimiter` interface, so most alternatives below are contained
changes that don't touch the HTTP layer.

## Tradeoffs we made

### Leaky bucket (smoothing), not a strict window
The leaky bucket allows a burst of up to 10 then refills ~1 slot every 6s, so the worst case
across a poorly-aligned 60s window is ~20, not a literal 10. This is standard, O(1), and
allocation-free. A **sliding-window log** gives a strict "≤10 in any rolling 60s" and is included
as a second strategy, selectable by config — at the cost of O(n) memory. Fixed-window and token
bucket sit in between; token bucket is functionally equivalent here.

### Global, not per-client
The requirement is a single global limit, so there is one shared bucket and no client key.
Per-client limiting would key buckets by identity (`userId`/IP) in a map behind the same
interface, plus idle-entry eviction to bound memory. The HTTP layer would be unchanged.

### Limiter before auth
The limiter is the first thing on `/api`, ahead of authentication, so it's a true front door:
malformed/anonymous floods can't consume auth work, and the Go and Java pipelines behave
identically. The cost is that a single anonymous client can exhaust the global budget for everyone
— inherent to any global limit. Counting only authenticated traffic is a one-line reorder (place
it after the auth check).

### `/health` exempt by routing
Health checks and load balancers must never be throttled. Exemption is structural — `/health`
lives outside the `/api` subtree the limiter guards — rather than a special case inside the
limiter.

### One global mutex
A single `sync.Mutex` wraps the entire read-modify-write. The critical section is a few arithmetic
operations with no I/O, so contention is negligible at realistic load, and correctness is simple
to reason about (verified under `-race`, including a test that advances the clock under
contention). If throughput ever demanded it, the buckets could be sharded or moved to an
atomic-CAS design; that's premature here and the two-field float state doesn't pack cleanly into
one atomic word.

### Process-local, in-memory
No distributed state, per the brief. Across multiple instances each has its own bucket, so N
instances admit N×10/min in aggregate.

### Injected clock
Time is injected rather than read from `time.Now()` directly, which makes every time-based test
deterministic (no sleeps) and lets the limiter clamp elapsed time to ≥ 0 as defense against a
backward clock step.

## Future work / extensions

- **Per-client limiting** — key buckets by client identity with idle eviction.
- **Distributed limiting** — shared counter in Redis (`INCR` + TTL, or a token-bucket Lua script)
  for a cluster-wide limit across instances.
- **Metrics** — a rejection counter and current-level gauge (Prometheus/OpenTelemetry) in the
  middleware, which already holds the `Decision`.
- **Runtime reconfiguration** — swap the limiter behind an atomic pointer to change limits without
  a restart.
- **More strategies** — token bucket and fixed window drop in behind the existing interface and
  factory.
- **Concurrency limiting** — capping simultaneous in-flight (e.g. slow) requests is a separate
  concern (a semaphore/bulkhead), complementary to this rate limiter.
