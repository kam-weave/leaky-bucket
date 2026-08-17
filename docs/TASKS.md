# Task Log — Leaky Bucket Rate Limiter Interview

A running, chronological log of everything we did in this repo, so it can be talked through in the interview. Newest work appended at the bottom of each section.

## Assignment

> Implement a basic request rate limiting solution using a **Leaky Bucket** algorithm.
>
> **Requirements**
> - Implement a process-local rate limiter (no distributed state required).
> - The rate limit should apply **globally** across all clients.
> - Configure the limiter to allow **10 requests per minute**.
> - Keep the implementation clean, readable, and maintainable.
> - Include any tests or documentation you feel are appropriate.

## Working agreement (how we're running this)

- **Test-first / red-green.** Every unit of work: write a failing test, commit it, then make the change that turns it green, commit that. Small single-purpose commits telling the story red → green → refactor.
- **Context first, code later.** Before touching the limiter we ingest and document how both apps are architected so any AI (or human) picking this up next is oriented fast.
- **Two implementations exist** (Go + Java) with identical APIs. We will pick one to implement in (TBD with interviewer/user) but document where the limiter plugs into *both*.

---

## Phase 1 — Repo ingestion & context building

- [x] Explored repo layout (`go/`, `java/`, top-level `Makefile`, `README.md`).
- [x] Spun up parallel sub-agents to deep-read the Go and Java implementations in parallel.
- [x] Wrote `docs/architecture-go.md` — Go request pipeline & where the limiter plugs in.
- [x] Wrote `docs/architecture-java.md` — Java request pipeline & where the limiter plugs in.
- [x] Wrote `docs/rate-limiter-plan.md` — the Leaky Bucket design & red-green implementation plan.
- [x] Added inline **Mermaid** diagrams (render natively on GitHub) to the three docs:
  request pipelines (Go, Java) + the leaky-bucket decision flow. Embedded, not a separate
  file, to avoid sprawl.
- [x] Added `CLAUDE.md` (AI/dev onboarding → points to `docs/`) and a README pointer.
- [x] Added one-liner local-preview setup: `make setup` installs the `bierner.markdown-mermaid`
  VS Code extension; `.vscode/extensions.json` prompts it on repo open. Documented in
  `CLAUDE.md` + README.
- [x] Verified diagrams render (validated Go diagram via mermaid-cli → clean SVG). Documented
  the one gotcha: installing the extension via `make setup` while VS Code is already open
  needs a one-time "Reload Window" before the preview picks it up.
- [x] Fixed diagrams going blank in VS Code **dark** theme (rendered then vanished) by pinning
  `%%{init: {'theme':'default'}}%%` in each diagram — theme-independent, also correct on GitHub.
- [x] The VS Code extension still rendered blank on this machine (its own dark-mode bug), so
  added an **extension-free** viewer: `make diagrams` (`scripts/render-diagrams.sh`) renders all
  `docs/` Mermaid to SVG (via `npx mermaid-cli`) into gitignored `build/diagrams/` and opens a
  browser gallery. This is now the recommended local path; GitHub remains primary for the PR.
- [x] **Fixed a docs-consistency bug:** the Go and Java pipeline diagrams described the *same*
  design decision (`/health` is exempt) differently — Go drew an explicit `/health` branch,
  Java only mentioned "skips /health" inside a box. Reworked the Java diagram to show the same
  explicit branch. Lesson: with parallel docs per implementation, a shared design decision must
  be depicted identically in each; the underlying *mechanism* can differ (Go exempts `/health`
  by routing outside `/api`; Java by a path check in the filter) but the *decision* must read
  the same. Worth calling out live as an example of keeping parallel docs in sync.

### Key findings (talking points)

- **Both apps have an identical, layered request pipeline** with a clear global-middleware seam
  to hook into — no code needs restructuring to add a limiter.
- **Go:** limiter is a `func(http.Handler) http.Handler` middleware added via `r.Use(...)` in
  `go/internal/app/app.go` (top-level chain for global incl. `/health`, or inside the
  `/api` subtree to exempt `/health`). One shared instance created in `NewRouter`.
- **Java:** cleanest fit is a `@Component @Order(2)` servlet `Filter` mirroring `AuthFilter`
  (auto-registered, no `WebConfig` change); alternatively a `HandlerInterceptor` scoped to
  `/api/**`.
- **Neither app has existing shared mutable in-memory state or a rate-limit library** — our
  leaky bucket is hand-rolled and is the first such state, so it owns its own thread safety
  (mutex/lock guarding the bucket level).
- **Tests:** Go uses `httptest.NewServer` + an `authedRequest` helper in `go/api_test.go`;
  Java uses `@SpringBootTest` and we'll add `@AutoConfigureMockMvc`. In both, prefer
  unit-testing the limiter core with an **injected clock** (no real sleeps) and keep the HTTP
  test to a single "429 after the limit" assertion.

- [x] Added **TypeScript equivalents** throughout `docs/rate-limiter-plan.md` (reasoning aid for
  a TS-native reader — TS is *not* a build target; scope stays Go + Java):
  - a `LeakyBucket` reference class with an injected `Clock` (`() => number`);
  - the key concurrency contrast — Go/Java need a mutex/lock across request threads, Node's
    single-threaded event loop does not, *as long as `tryAcquire()` stays synchronous*;
  - an Express/Connect `rateLimit` middleware scoped to `/api` (Fastify `preHandler` noted),
    mirroring the Go/Java `/health`-exempt placement;
  - test notes mapping the injected clock across Go / Java / TS (incl. Jest fake timers).
- [x] Hardened the TS design for the **global-singleton** invariant: split *mechanism*
  (freely-constructable `LeakyBucket`, for tests only) from the *app-wide singleton*
  (`rate-limiter.ts` constructs the one bucket and exports only the middleware; the class isn't
  re-exported, so app code can't `new` its own). Documented the private-constructor/static
  accessor alternative and why the module-singleton is the cleaner analog to how Go/Java wire
  a single instance at composition time.
- [x] Extended the **single global instance** invariant to Go and Java in the plan (§3), so all
  three stacks enforce it explicitly, each idiomatically: Go → construct once at the composition
  root (`NewRouter`) + constructor injection, unexported fields, `sync.Once` only if a hard
  global is wanted; Java → rely on Spring's default **singleton-scoped bean** + constructor
  injection with a package-private constructor. Framed as a deliberate design property to raise
  in the interview.

### Adversarial plan review (before writing code)

- [x] Spun up **3 parallel adversarial review agents** against the plan + architecture docs,
  each with a distinct lens, to surface gaps before implementation:
  1. **Algorithm correctness** — leak math, off-by-one, "10/min" literal vs burst behavior,
     clock/precision, leaky vs token-bucket fit.
  2. **Concurrency & single-instance** — lock coverage of read-modify-write, singleton
     airtightness across Go/Java/TS, event-loop-as-mutex claim, contention.
  3. **HTTP/API/testability & the plan itself** — 429 body/headers, exemptions, injected-clock
     specificity, the shared-server test-isolation risk, red→green ordering.
- [x] Triaged all three reports and folded accepted fixes into the plan (§5 decisions, new §6
  "Adversarial review — findings & resolutions") and reconciled the architecture docs.

#### Review findings & what we did (interview talking points)

The red-team caught **real design bugs, not nitpicks** — good story about using AI adversarially:

1. **"10 req/min" was overstated.** Leaky-bucket smoothing permits burst 10 + ~10 leaked ≈ **20**
   in a bad 60s window, not 10. → §2 now states the exact guarantee + a model comparison table
   (only a sliding-window log gives a strict 10/60s). Flagged as an open decision for the grader.
2. **Non-atomic read-modify-write risk.** leak→check→increment must be **one** locked critical
   section or two requests both see `level=9` and both pass. → mandated in §6.2 + both arch docs.
3. **Two stacks behaved differently around auth.** Go limited *after* `RequireAuth` (anon
   requests never counted); Java's filter ran *before* auth (anon requests consumed tokens). →
   unified: limiter is the front door of `/api` in **both**, before auth; anon counts.
4. **Java raw `Filter` could double-count** on async re-dispatch. → switch to
   `OncePerRequestFilter` + `FilterRegistrationBean` (dispatch `REQUEST` only).
5. **Shared Go test harness = flaky limiter tests + throttled benchmarks.** One `testServer`/
   bucket leaks state across the whole binary. → limiter HTTP tests build their own
   router+bucket+clock; benchmarks relax the limit via config.
6. **Clock hazards.** Wall-clock (`Instant`) can jump backward → spurious rejects; Go/Java could
   disagree. → monotonic where possible, clamp `elapsed ≥ 0`, one fixed clock signature + time
   unit per stack.
7. **429 contract under-specified.** → `Retry-After` + stable error body are now required, not
   optional. Plus: log rejections, don't read `userId`, surface capacity/period as config.
8. **Red→green steps too coarse.** → split into one-assertion steps, config-shape first.

- [x] Added plan **§7 Interpretation & live-evolution path** to handle the brief's ambiguity
  ("10/min" + "Leaky Bucket", no further spec) and the expectation that we evolve it live:
  - **meter (reject) vs shaper (queue):** we build the *meter* — reject with 429, no queue, so
    request duration is irrelevant and nothing backs up (the shaper is the variant with
    queue-backup risk; not building it unless asked);
  - **"naive vs smoothed" is an algorithm switch:** naive 10/min = fixed-window counter (a
    different algorithm); our leaky bucket already *is* the smoothed form;
  - **rate-limiting ≠ concurrency-limiting** (semaphore/bulkhead is the tool for slow-request
    pileups);
  - a pivot table: keep the limiter behind a one-method `tryAcquire()` seam so fixed-window ↔
    leaky ↔ token ↔ shaper ↔ per-client are each a contained, one-file swap during the live
    session.
- [x] Added plan **§8 Extensibility — the `RateLimiter` interface (Open/Closed)**: the algorithm
  sits behind a Strategy interface (`Allow()/allow()` returning a `Decision{allowed, retryAfter}`)
  so the middleware/filter and wiring depend on the interface, never a concrete algorithm.
  Selection is config-driven at the composition root via a small factory (Go `switch`; Java
  `@ConfigurationProperties` + factory/`@ConditionalOnProperty`). Adding an algorithm = new
  type + one factory case, zero edits to existing code (closed to modification, open to
  extension). Red→green gets an interface-first step 0 and a later "second strategy +
  factory-selection test" step to prove OCP.
- [x] **Confirmed the second strategy: `SlidingWindowLog`** (plan §8.1) — the one algorithm that
  gives the *literal* "≤10 per rolling 60s" (evict timestamps older than the window; accept if
  fewer than 10 remain; `Retry-After` = when the oldest ages out). Trade-off: exact but O(n)
  timestamps (trivial at limit 10) and no burst-smoothing. Ships alongside `LeakyBucket` (default)
  behind the same `RateLimiter` interface + clock harness, so switching strict↔smoothed is a
  config flip, not a rebuild. Reflected in §2 table, §6.1, §7 pivot table, §8.1, and red→green.
- [ ] **Open decisions for the user** (plan §6.1): (a) strict vs smoothed is now a **config flip**
  (`SlidingWindowLog` pre-built) — default is `LeakyBucket`; (b) do unauthenticated `/api`
  requests count (plan says yes); (c) exemptions beyond `/health`. Leaning: ship both strategies
  behind the seam, default to leaky bucket, let the interviewer steer the rest live.

- [x] **Simplified `rate-limiter-plan.md`** (351 → ~170 lines) before building: merged the
  overlapping sections (meter-vs-shaper appeared in both §2 and §7; decisions duplicated across
  §5/§6), led with a compact **confirmed-decisions table**, trimmed the oversized TS singleton
  code to a one-line mental-model note, and folded the adversarial findings into a short
  talking-points appendix. Structure is now: Assignment → Decisions → Algorithms (leaky +
  sliding-window) → Architecture (interface/placement/concurrency/clock) → Build plan → Appendix.
  Content/decisions preserved; diagram still renders.

## Phase 2 — Implementation (Go first)

Branch `rate-limiter-go`. Sequential red→green: each test below is one failing-test commit
followed by one make-it-pass commit. Package: `go/internal/ratelimit`.

### Test checklist (itemized before building)

**Strategy 1 — `LeakyBucket` (default), unit tests with an injected clock:**
- [x] **T1 — allow N, reject N+1:** 10 immediate `Allow()` calls return allowed; the 11th is
  rejected (boundary `level == capacity`).
- [x] **T2 — leak frees a slot:** fill to capacity, advance the fake clock past one leak interval
  (~6s), next `Allow()` is allowed (proves leak-then-check ordering).
- [x] **T3 — partial leak is proportional:** advance < one interval → still rejected; advance ≥
  one interval → exactly one slot frees (pins the leak-rate math & units).
- [x] **T4 — rejects don't consume:** a burst of rejections doesn't push recovery out; `last`
  advances on reject; after waiting, exactly the expected number free up.
- [x] **T5 — full recovery / cap:** after a long idle, `level` floors at 0 (not negative) and the
  next burst of 10 is allowed again; level never exceeds capacity.
- [x] **T6 — Retry-After:** on reject, `Decision.RetryAfter` ≈ time until one slot frees (~6s),
  and shrinks as the clock advances.
- [x] **T7 — thread-safety:** N goroutines issuing M calls never allow more than capacity within
  a frozen-clock window (`-race`); smoke test for the single-critical-section invariant.

**HTTP integration (dedicated router + injected clock, NOT the shared testServer):**
- [x] **T8 — 429 after burst:** 11th request to an `/api` route → `429` with `Retry-After`
  header and the `{"error": ...}` JSON body shape.
- [x] **T9 — /health exempt:** many `/health` requests never 429.
- [x] **T10 — unauthenticated still counts:** requests without a token count against the bucket
  (limiter sits before auth) — 429 before 401 once the bucket is full.

**Strategy 2 — `SlidingWindowLog` (strict ≤10/rolling-60s), same interface:**
- [x] **T11 — allow 10 / reject 11th** within the window.
- [x] **T12 — oldest ages out:** advance clock so the oldest timestamp exits the 60s window →
  one slot frees.
- [x] **T13 — strict rolling window:** spread 10 across the window, then a burst at the end still
  can't exceed 10 in any 60s span (the property leaky bucket fails). _(Note: first draft of this
  test placed all 10 at the same instant so they aged out together — a test bug; fixed by spacing
  them 1s apart. Implementation was already correct.)_
- [x] **T14 — Retry-After:** ≈ `60s - (now - oldest)`.
- [x] **T15 — factory selection:** `algorithm = "sliding_window_log"` config yields a
  `SlidingWindowLog`; default yields `LeakyBucket` (proves OCP wiring).

### Build log (test → implementation, one row per red→green pair)

_Appended as we complete each step._

| # | Test | Implementation that made it pass |
|---|------|----------------------------------|
| T1 | allow 10, reject 11th | `ratelimit.go` (`Clock`, `Decision`, `RateLimiter`) + `leakybucket.go` (`NewLeakyBucket`, mutex-guarded leak→check→increment `Allow()`) |
| T2 | leak frees one slot per interval | no code change — locks the leak-then-check behavior from T1 |
| T3 | partial leak is proportional | no code change — pins the leak rate/units |
| T4 | rejections don't consume capacity | no code change — level only rises on accept |
| T5 | floor at 0 + full recovery after idle | no code change — `level < 0` clamp from T1 |
| T6 | Retry-After on reject | `leakybucket.go`: reject branch now returns `RetryAfter = deficit/leakPerNano` (time until level drops to capacity-1) |
| T7 | concurrent never exceeds capacity (`-race`) | no code change — verified the `sync.Mutex` critical section is clean under the race detector |
| T8 | HTTP 429 + Retry-After after burst | `middleware.go` (429 + Retry-After header + JSON body); `app.go` (`Options.RateLimiter`, applied first inside `/api` before auth); `main.go` wires the real 10/min limiter; isolated `app_test.go` harness |
| T9 | /health exempt | no code change — limiter lives inside the `/api` subtree, `/health` is outside it |
| T10 | unauthenticated requests count | no code change — confirms the before-auth placement (429 preempts 401 once full) |
| T11 | sliding window allows 10, rejects 11th | `slidingwindow.go` — `NewSlidingWindowLog`, mutex-guarded evict→count→append `Allow()` (same `RateLimiter` interface) |
| T12 | window expiry frees capacity | no code change — locks the eviction logic from T11 |
| T13 | strict rolling window (≤10 in any 60s) | no impl change — test corrected to space requests 1s apart; proves the property leaky bucket can't |
| T14 | sliding window Retry-After | `slidingwindow.go`: reject branch returns `window - (now - oldest)` |
| T15 | config-driven algorithm selection | `factory.go` (`Config`, `New()` switch, `Algorithm*` constants); `main.go` builds the limiter via the factory |

**Status:** T1–T15 complete. `go vet` clean, `go test -race ./...` green across all packages;
existing benchmarks unaffected (limiter is nil in the shared test harness). Files added:
`go/internal/ratelimit/{ratelimit,leakybucket,slidingwindow,middleware,factory}.go` +
`ratelimit_test.go`; `go/internal/app/app_test.go`; wired via `app.Options.RateLimiter` and
`main.go`.

### Adversarial code validation (post-implementation)

- [x] Ran 3 adversarial agents (correctness, concurrency, HTTP/API) against the finished Go code.
  **Verdict from all three: correct for the required 10/min config — no live bugs.** (Correctness
  agent even verified the leak math lands on exactly 9.0 after 6s; concurrency agent confirmed a
  single critical section in both limiters, true singleton, no lock held during I/O; HTTP agent
  confirmed placement/exemption/before-auth/single-instance and that the seed & benchmark paths
  are unaffected.) Findings were defensive hardening + test-coverage gaps:

| # | Finding | Action |
|---|---------|--------|
| F1 | Factory/constructors didn't reject `Limit<=0`/`Period<=0` (Period 0 → +Inf leak silently disables the limiter) | **Fixed** — validate in `New` |
| F2 | `SlidingWindowLog` lacked the backward-clock clamp `LeakyBucket` has | **Fixed** — clamp Retry-After to `[0, window]` |
| F3 | Concurrency test froze the clock, so the leak/eviction mutation path never ran under contention; sliding had no concurrency test | **Fixed** — added advancing-atomic-clock concurrent tests (bound assertions) for both |
| F4 | HTTP tests didn't assert the Retry-After value or the JSON body, and never advanced the clock at the HTTP layer | **Fixed** — added asserts + an HTTP recovery test |
| F5 | Plan wanted rejections logged; middleware logged nothing | **Fixed** — `slog.Warn` on reject |
| — | Per-client vs global (spec says global) | not a bug — noted talking point |
| G1 | DRY: 429 body duplicated the API error envelope | **Fixed** — shared `apierr.Write` |
| G2 | No `RateLimit-*` headers | **Fixed** — emit Limit/Remaining/Reset |
| G3 | Pre-existing 401 was plain-text, not JSON | **Fixed** — 401 now uses `apierr.Write` |

**Follow-up fix commits (user asked to also address the deferred items):**

| Fix | Test | Change |
|-----|------|--------|
| G1 | existing (F4 body assert) | new `internal/apierr` package; `api.writeError` and rate-limit middleware both delegate to `apierr.Write` |
| G2 | `TestHTTP_RateLimitHeaders` | `Decision` gains `Limit`/`Remaining` (set by both limiters); middleware emits `RateLimit-Limit`/`-Remaining` on every /api response and `-Reset` on 429 |
| G3 | `TestHTTP_UnauthorizedIsJSON` | `auth.RequireAuth` now returns the JSON `{"error":...}` envelope via `apierr.Write` (was plain text) |

**Fix commits (same one-per-item pattern):**

| Fix | Test | Change |
|-----|------|--------|
| F1 | `TestFactory_RejectsDegenerateConfig` | `factory.go` validates `Limit>0` && `Period>0` |
| F2 | `TestSlidingWindow_BackwardClockClampsRetryAfter` | `slidingwindow.go` clamps Retry-After to `[0, window]` |
| F3 | `TestLeakyBucket_ConcurrentWithAdvancingClock`, `TestSlidingWindow_ConcurrentWithAdvancingClock` | `atomicClock` helper; advance clock per call so leak/eviction runs under contention (bound assertions, `-race`) |
| F4 | `TestHTTP_RetryAfterValueBodyAndRecovery` | asserts exact `Retry-After: 6`, JSON `{"error":...}` body, and clock-driven recovery (no code change) |
| F5 | `TestMiddleware_LogsRejectionOnly` | `middleware.go` logs rejections via injected `*slog.Logger` (warn; allowed requests silent); `app.go` passes nil → `slog.Default()` |
