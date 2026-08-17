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

## Mutation

**Why:** line coverage tells you which code *ran* during tests, not whether the tests would
*notice if it broke*. A suite can execute 100% of the limiter yet still pass if you flip `<=` to
`<` or delete an increment. Mutation testing closes that gap: it programmatically introduces small
faults ("mutants") — flip a comparison, change `+` to `-`, remove a statement — reruns the tests,
and reports how many mutants were **killed** (a test failed, good) vs **survived** (no test
noticed, a real gap). It measures the *strength* of the tests, which is what actually protects the
code.

**Tool:** [gremlins](https://gremlins.dev), scoped to `go/internal/ratelimit/`.

```bash
make test-mutation
```

**What it found here (a concrete example of the value):** the first run scored **72.5%** efficacy.
The survivors were genuine gaps — the limiters' unit tests never asserted `Decision.Remaining`
(only the HTTP layer did), and nothing exercised the middleware's "round Retry-After up to at
least 1 second" floor. We added two targeted tests (`TestDecision_LimitAndRemaining`,
`TestMiddleware_RetryAfterFlooredToOne`), which raised efficacy to **~85%**. The handful of
remaining survivors are equivalent/boundary mutants (e.g. `>` vs `>=` on a clamp that produces the
same result) — not worth chasing. The point isn't a perfect score; it's that mutation testing
*drove real test improvements that coverage alone would never have surfaced.*

**Running it reliably:** `make test-mutation` sets `GOFLAGS=-count=1`. Without it, Go's cached
test results make gremlins measure a near-zero baseline time and then time out every mutant that
must recompile — a subtle interaction worth knowing about when wiring mutation testing into a Go
project.
