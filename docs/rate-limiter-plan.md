# Leaky Bucket Rate Limiter — Design & Implementation Plan

## 1. What the assignment asks for

- Process-local (in-memory, single process) — **no Redis/distributed state**.
- **Global** limit — one shared bucket for *all* clients, not per-IP/per-token.
- **10 requests / minute**.
- Clean, readable, maintainable; tests + docs.

## 2. Leaky Bucket, the two variants

The "leaky bucket" name covers two closely related models:

1. **Leaky bucket as a queue (meter/shaper).** Requests enter a FIFO queue (the bucket).
   They *leak out* (are processed) at a fixed rate. If the bucket is full, new requests
   are dropped/rejected. This *smooths* bursts into a constant output rate.
2. **Leaky bucket as a counter (the common web-rate-limit form).** A counter represents
   the current "water level." Each request adds 1 unit. The bucket leaks continuously at
   a constant rate (capacity / period). If adding 1 would overflow capacity, reject.

For an HTTP rate limiter we want variant **2** (the counter form) — it's the standard,
allocation-free, O(1) approach and maps directly to "N requests per minute."

```mermaid
%%{init: {'theme':'default'}}%%
flowchart TD
    r([Request to /api/**]) --> leak["Leak: level -= elapsed × rate<br/>(floor at 0)"]
    leak --> chk{"level + 1 ≤ capacity?<br/>(capacity = 10)"}
    chk -- yes --> ok["level += 1<br/>→ allow (200)"]
    chk -- no --> rej["reject → 429<br/>Too Many Requests"]
    style rej fill:#ffcdd2,stroke:#b71c1c
    style ok fill:#c8e6c9,stroke:#1b5e20
```

One shared, mutex-guarded bucket serves all clients (the global limit). Steps below.

### Chosen model (counter form)

- `capacity = 10` (max water the bucket holds → burst ceiling).
- `leakRatePerSecond = capacity / 60 = 10/60 ≈ 0.1667` units/sec.
- On each request:
  1. Compute elapsed time since last update; leak `elapsed * leakRate` units (floor the
     level at 0).
  2. If `level + 1 <= capacity`: accept, `level += 1`.
  3. Else: reject with **HTTP 429 Too Many Requests**.
- One shared instance guarded by a mutex → satisfies "global across all clients."

This allows a burst of up to 10 immediately, then drips back one slot roughly every 6s,
averaging 10/min. (We'll note the alternative "strict fixed window" behavior in the doc
and explain why leaky bucket's smoothing is preferable.)

## 3. Where it plugs in

- **Go:** a middleware wrapping the API mux — see `docs/architecture-go.md` (§ request
  pipeline). Returns 429 before the handler runs.
- **Java:** a servlet `Filter` (or `HandlerInterceptor`) registered globally — see
  `docs/architecture-java.md`.

_(Exact file/line insertion points filled in from the architecture docs.)_

## 4. Red → Green implementation plan (single small commits)

Each numbered item = one failing-test commit followed by one make-it-pass commit.

1. **Core limiter unit tests** (no HTTP):
   - allows the first 10 requests, rejects the 11th within the same minute.
   - after leaking (advance a fake clock), a slot frees up and the next request is allowed.
   - concurrency: N goroutines/threads hammering the limiter never exceed capacity.
   - Inject a clock (function/interface) so time is deterministic in tests — **no `sleep`**.
2. **Config**: limit expressed as `capacity=10, per=1 minute`, easy to change in one place.
3. **HTTP integration**: middleware/filter returns 429 with a sensible body/headers once
   the global limit is exceeded; health check (`/health`) behavior decided (likely exempt).
4. **Wire into the app** and add an end-to-end test hitting the real routes.
5. **Docs**: short README section on the limiter + tuning knobs.

## 5. Decisions (confirmed)

- **Scope: both Go and Java.** Implement the limiter in both stacks with identical behavior;
  each gets its own red→green commit sequence.
- **`/health` is exempt.** The bucket only applies to `/api/**`, so health checks never trip
  the limit. Concretely: Go → add the middleware inside the `r.Route("/api", ...)` subtree
  (not the top-level chain); Java → `HandlerInterceptor` scoped to `/api/**`, or a
  `Filter` that skips paths not under `/api/`.

### Still to confirm (lower stakes, can decide at implementation time)

- Response on limit: plain **429** body; optionally add `Retry-After` / `X-RateLimit-*`
  headers (nice-to-have, not required).
- Bursting: leaky-bucket smoothing allows an initial burst of 10 then drips ~1 slot / 6s.
  We'll go with this (standard leaky-bucket behavior) unless a strict fixed window is wanted.
