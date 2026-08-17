# Testing Strategy

The rate limiter is tested at four levels. Each answers a different question, and each catches
failures the others can't — so they're complementary, not redundant.

| Level | Location | Question it answers | Speed |
|-------|----------|---------------------|-------|
| **Unit** | `go/internal/ratelimit/*_test.go` | Is the algorithm correct in isolation? | ms |
| **Integration** | `go/internal/app/*_test.go` | Do the limiter, router, auth, and store work together? | ~ms |
| **E2E** | `go/e2e/` (build tag `e2e`) | Does the real built server behave correctly as a process? | seconds |
| **Mutation** | `make test-mutation` | Are the tests themselves strong enough to catch bugs? | minutes |

## Unit

Test the leaky-bucket and sliding-window algorithms directly, with an **injected clock** so time
is advanced deterministically (no `sleep`). Covers admission (allow N / reject N+1), leaking and
recovery, Retry-After, and thread-safety under `-race`. Fast and exhaustive on edge cases; blind
to how the limiter is wired into HTTP.

## Integration

Build the **real router** (`app.NewRouter`) with a real store, the auth middleware, and the
limiter, and drive it over in-process HTTP (`httptest.NewServer`). This is where cross-cutting
behavior is verified: that one global bucket is shared across *different* endpoints, that
`/health` is exempt and unauthenticated requests count, that `RateLimit-*` headers appear on real
endpoint responses, and that swapping to the sliding-window strategy works end-to-end through the
router. The clock is still injected, so time-based behavior (recovery) stays deterministic. It
does **not** exercise `main.go`, process startup, or a real network socket — that's E2E.

## E2E

Build the **real server binary**, seed a temporary SQLite database, run it as an OS process, and
drive it over a **real HTTP socket**. This is the only test that exercises `main.go` (the real
limiter built via the factory with the wall clock), process startup/readiness, and a genuine
network round-trip — things every lower level stubs out. Because it starts a process it's slower,
so it's behind the `e2e` build tag and a separate target:

```bash
make test-e2e          # or: go test -tags e2e ./e2e/...
```

The burst of 10 → 429 is asserted without any waiting (the burst is instantaneous); testing
time-based recovery is left to the unit/integration levels where the clock is injectable.
