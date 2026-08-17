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
- On each request (the **whole** block is one atomic critical section — see §6.2):
  1. `elapsed = max(0, now - last)` (clamp so a backward clock can't inflate the level);
     leak `elapsed * leakRate`; `level = max(0, level - leaked)`; set `last = now`. **Update
     `last` on every call**, accept or reject.
  2. If `level + 1 <= capacity`: accept, `level += 1`. (Rejected requests do **not** consume
     capacity — `level` only rises on accept.)
  3. Else: reject with **HTTP 429 Too Many Requests**.
- Invariant: `0 ≤ level ≤ capacity` at all times.

**Exact guarantee (state this; don't oversell "10/min").** This is *leaky-bucket smoothing*,
not a strict rolling window: it permits a **burst of 10, then ~1 slot every 6s**. Worst case
across a poorly-aligned 60s window is ~**20** (10 burst + ~10 leaked), not 10. Off-by-one is
deliberate: the 10th request is allowed, the 11th (at `level == capacity`) is rejected.

| Model | Worst case / 60s | Note |
|-------|------------------|------|
| Fixed window | up to ~20 at boundary | naive "per minute" |
| Sliding-window log | exactly 10 | only literal-correct option; O(n) memory |
| **Leaky bucket (chosen)** | ~20 | smooth drip, O(1), allocation-free |
| Token bucket | ~20 | equivalent here |

If the grader requires a strict "never >10 in any 60s window," only a sliding-window log
delivers it — leaky/token buckets cannot. **Open decision (§6.1):** confirm burst-smoothing is
acceptable.

### TypeScript equivalent (mental model)

The same counter form in idiomatic TS, with the clock injected for deterministic tests:

```ts
type Clock = () => number; // epoch millis

export class LeakyBucket {
  private level = 0;
  private last: number;

  constructor(
    private readonly capacity: number,   // 10
    private readonly ratePerMs: number,  // capacity / 60_000  (leak per ms)
    private readonly now: Clock = Date.now,
  ) {
    this.last = now();
  }

  /** Consumes one slot and returns true if the request is allowed. */
  tryAcquire(): boolean {
    const t = this.now();
    this.level = Math.max(0, this.level - (t - this.last) * this.ratePerMs);
    this.last = t;
    if (this.level + 1 <= this.capacity) {
      this.level += 1;
      return true;
    }
    return false;
  }
}
```

**Key concurrency difference vs. Go/Java.** Go serves each request on its own goroutine and
Java on its own Tomcat worker thread, so the shared bucket **must** be guarded by a
`sync.Mutex` / lock. Node.js runs a **single-threaded event loop**, so `tryAcquire()` — being
fully synchronous with no `await` between the read and the write — runs to completion without
interleaving. **No lock is needed** as long as that method never becomes `async`. That's the TS
mental model: the event loop *is* your mutex, but only across synchronous sections.

## 3. Where it plugs in

All three stacks must guarantee the **single global instance** — one bucket for the whole
process. Each enforces it idiomatically:

- **Go:** a middleware wrapping the API mux — see `docs/architecture-go.md` (§ request
  pipeline). Returns 429 before the handler runs.
  - *Single-instance enforcement:* construct the bucket **once at the composition root**
    (`NewRouter`) and capture it in the middleware closure — the same place the router and
    store are already wired. Keep the struct fields unexported and expose only
    `NewLeakyBucket(...) *LeakyBucket` + a `tryAcquire`-style method, so state can't be mutated
    from outside. Prefer this constructor-injection over a package-level global; if a hard
    process-wide singleton is ever wanted, wrap creation in `sync.Once`, but injection stays
    more testable (each test builds its own bucket with a fake clock). The router is itself
    built once in `main`, so exactly one bucket exists.
- **Java:** a servlet `Filter` (or `HandlerInterceptor`) registered globally — see
  `docs/architecture-java.md`.
  - *Single-instance enforcement:* Spring beans are **singleton-scoped by default**, so
    declaring the limiter a single `@Component` (or `@Bean`) means the container creates exactly
    one instance and injects that same object everywhere. That's the framework enforcing the
    invariant for us. Harden it by giving the filter the bucket via **constructor injection**
    (no `new` in app code) and keeping any manual constructor package-private so nothing outside
    can build a second one. Unit tests still `new` an isolated bucket directly with a fake
    clock.
- **TypeScript (equivalent):** the limit is **global**, so the app must hold exactly one
  bucket — the TS design has to enforce that, the same way Go creates it once in `NewRouter`
  and Java registers a single `@Component` bean. Split the two concerns:

  **1. Mechanism** (`leaky-bucket.ts`) — the `LeakyBucket` class above stays freely
  constructable, but *only so unit tests can spin up isolated instances with a fake clock*.
  App code never imports it directly.

  **2. App-wide singleton** (`rate-limiter.ts`) — construct the one shared bucket here and
  export only the middleware. Node caches modules, so this instance is created once and every
  importer shares it; because the class isn't re-exported, app code *cannot* `new` its own:
  ```ts
  import { LeakyBucket } from "./leaky-bucket";

  const bucket = new LeakyBucket(10, 10 / 60_000); // the ONE global bucket

  export const rateLimit: RequestHandler = (_req, res, next) =>
    bucket.tryAcquire() ? next() : res.status(429).json({ error: "rate limit exceeded" });
  // app.use("/api", rateLimit) → scoped to /api, so /health stays exempt (mirrors Go/Java).
  ```
  Fastify would be the analogous `preHandler` hook on the `/api` scope.

  (A private-constructor + static-accessor singleton would enforce this *within* a module too,
  but it bakes the clock/config in at first call — worse for testing — so the module-singleton
  above is the cleaner analog to how Go/Java wire the single instance at composition time.)

> **Scope reminder:** TypeScript here is a *reasoning aid only* — not a deliverable. Ship Go +
> Java. Don't port the TS singleton patterns into either.

_(Exact file/line insertion points filled in from the architecture docs.)_

## 4. Red → Green implementation plan (single small commits)

One failing-test commit → one make-it-pass commit, **one assertion per step**. Step 0 also
introduces the `RateLimiter` interface (§8) so the middleware programs to the interface from the
first commit; `LeakyBucket` is the first implementation. Config comes first because the core test
needs the `(capacity, period)` constructor to exist. The clock is
injected so time is deterministic — **no real `sleep`**. Fixed per-stack signatures:
**Go `type Clock func() time.Time`** (elapsed via `.Sub`, monotonic); **Java `java.time.Clock`**
(`clock.instant()`), with elapsed clamped `>= 0` since `Instant` isn't monotonic.

0. **Config shape** — `(capacity, period)`; derive `rate = capacity/period` internally (one
   source of truth). Surface it on the existing config seam (Go `app.Options`; Java
   `@ConfigurationProperties`/`application.properties`), so tests/benchmarks can relax it.
1. **Allow N / reject N+1** — 10 immediate calls allowed, 11th rejected (at `level==capacity`).
2. **Leak frees a slot** — fill, advance the fake clock past one interval, next call allowed
   (proves *leak-then-check* ordering, not check-then-leak).
3. **Rejections don't consume** — a burst of rejects doesn't push recovery out; `last` advances
   on reject; leak resumes on schedule.
4. **Thread-safety** (Go/Java) — N goroutines/threads never exceed capacity (smoke test for the
   single-critical-section invariant, §6.2). _(TS: N/A — assert a synchronous 11-call loop = 10
   `true` then `false`.)_
5. **HTTP 429 contract** — build a **dedicated** router/server with a *fresh* bucket + injected
   clock (do **not** reuse the shared `testServer`; see §6.3). Assert 11th `/api` request →
   `429` **and** `Retry-After` header. `/health` never 429s.
6. **Wire into the app** + one end-to-end test on real routes.
7. **Docs** — short README section: the exact guarantee (§2), tuning knobs, exemptions.

## 5. Decisions (confirmed)

- **Scope: both Go and Java**, identical behavior; TS is a reasoning aid only.
- **`/health` is exempt** — limiter applies to `/api/**` only.
- **Placement is identical in both stacks** (resolves the auth-ordering divergence, §6.4):
  the limiter runs **as the first thing on `/api`, before auth** — so it is a true global API
  front door and unauthenticated requests **do** count. Go → `r.Use(rateLimit)` *before*
  `RequireAuth` inside the `/api` subtree; Java → keep the servlet filter but **scope it to
  `/api/**`** (skip other paths) so `/health` stays exempt.
- **Java uses `OncePerRequestFilter` + `FilterRegistrationBean`** (dispatch `REQUEST` only) so
  async re-dispatch can't double-count one request (§6.5).
- **429 body + `Retry-After`** are part of the contract, not optional: 429 with each stack's
  existing error-JSON shape (Go `writeError`, Java error body) plus `Retry-After: <seconds>`
  (~6s/slot). `X-RateLimit-Limit: 10` optional.
- **Rejections logged** at warn/debug (not per allowed request) so firing is observable.
- **The limiter never reads `userId`** — it's global; reading auth state invites accidental
  per-user logic.

## 6. Adversarial review — findings & resolutions

Three review agents (algorithm, concurrency, HTTP/API) red-teamed this plan pre-implementation.
Key issues and how the plan now answers them:

1. **"10/min" overstated** → §2 states the exact guarantee (burst 10 + ~10 leaked ≈ 20/60s) and
   the model comparison. **Open decision:** confirm burst-smoothing is acceptable vs a strict
   sliding window.
2. **Atomicity** → the *entire* leak→check→increment (incl. clock read) is **one** locked
   critical section: Go `mu.Lock()/defer Unlock()`, Java one `ReentrantLock`/`synchronized`
   covering **all** access to `level`/`last` (so no `volatile` needed). No lock-free fast path;
   no I/O or logging inside the lock.
3. **Shared Go test harness** → the single `testServer`/bucket in `TestMain` makes limiter state
   leak across tests and would 429 the benchmarks. **Resolution:** limiter HTTP tests build
   their own router+bucket+clock; benchmarks/other tests run with the limiter relaxed via the
   §4.0 config (high/effectively-infinite capacity).
4. **Auth-ordering divergence** (Go limited after auth, Java before) → unified in §5:
   limiter before auth in both; unauthenticated requests count.
5. **Java async double-count** → `OncePerRequestFilter`, dispatch `REQUEST` only (§5).
6. **Clock source** → monotonic where possible (Go `time.Now().Sub`; Java clamp elapsed ≥ 0);
   single fixed signature per stack (§4); one canonical time unit so Go/Java arithmetic agrees.
7. **HTTP-test clock seam** → provide a test-only `NewRouterWithClock` (Go) / `@TestConfiguration`
   overriding the `Clock` bean (Java) so the test controls the same clock the server uses;
   otherwise HTTP tests only assert the immediate burst→429 (state which).
8. **Single-lock contention** → acceptable: O(1) arithmetic critical section, no I/O; lock-free
   CAS rejected (two-field float state doesn't pack into one atomic word).

**Still open for the user (see §6.1):** (a) default is burst-smoothing (`LeakyBucket`); the
*strict* rolling-60s cap is pre-built as `SlidingWindowLog` (§8.1) and one config flip away, so
this is now a runtime choice, not a rebuild. (b) should unauthenticated `/api` requests count
(current plan: yes)?
(c) any endpoints beyond `/health` to exempt — e.g. CORS `OPTIONS` preflight, large
uploads/exports whose single call would lock out everyone for a minute?

## 7. Interpretation & live-evolution path

The brief gives only "**10 requests / minute**" + "**Leaky Bucket**." It leaves the exact
semantics open, and the interview is expected to evolve the design live. This section is the map
so we're never boxed in.

**A. Meter (reject) vs shaper (queue) — we build the meter.** On overflow we **reject
immediately with 429**; there is *no* queue, so request *duration* is irrelevant and nothing
"backs up." The **shaper** variant instead delays requests through a FIFO that leaks at a fixed
rate — which introduces queue depth, latency, and long-running-request backpressure. We're
deliberately not building the shaper unless asked; a fast 429 beats a hung connection for an API.

**B. "Naive" vs "smoothed" is an algorithm switch, not a knob.** A naive "10/min" is usually a
*fixed-window counter* (resets each minute; ~20 across a boundary) — a **different** algorithm.
Our leaky bucket already *is* the smoothed "burst 10 then drip" form; there is no simpler leaky
bucket. So "start naive, then add smoothing" really means fixed-window → leaky bucket.

**C. Rate limiting ≠ concurrency limiting.** We cap *admissions over time*, not *in-flight
requests*. Guarding against many simultaneous slow requests is a separate tool (semaphore /
bulkhead). Name this if the interviewer asks "what about a slow request?"

**Stance for the live session.** Ship the clean **meter-variant leaky bucket** first (on-brief,
tested), but keep it behind a **one-method seam** (`tryAcquire()` + the middleware/filter). That
makes every likely pivot a contained swap that touches neither the pipeline wiring nor the test
structure:

| If the interviewer wants… | What changes | Blast radius |
|---------------------------|--------------|--------------|
| Fixed-window first, then leaky | swap the algorithm behind `tryAcquire()` | one file |
| Shaper/queue (delay not reject) | `tryAcquire()` → an async `acquire()` that waits | limiter + handler await |
| Token bucket | swap the algorithm | one file |
| Per-client instead of global | key the buckets by client id (map) | limiter internals |
| Strict rolling-60s cap | flip config to the pre-built `SlidingWindowLog` (§8.1) | config value |

## 8. Extensibility — the `RateLimiter` interface (Open/Closed)

The pivots in §7 are only "one file" if we design for them now. So the algorithm sits behind a
small **interface** (Strategy pattern): the HTTP middleware/filter and the app wiring depend on
the *interface*, never a concrete algorithm. Adding an algorithm = **add a new type + one
factory case**; nothing existing is edited. That's Open/Closed: closed to modification, open to
extension.

**One shared shape across stacks** — a single method returning a decision that also carries
`Retry-After`, so the HTTP layer stays algorithm-agnostic (it never asks "which algorithm?"):

```go
// Go — an interface + struct implementations (Go has no classes)
type Decision struct {
    Allowed    bool
    RetryAfter time.Duration // set when !Allowed
}
type RateLimiter interface {
    Allow() Decision
}
// LeakyBucket, FixedWindow, TokenBucket, SlidingWindowLog each implement RateLimiter.
```

```java
// Java — interface + class implementations
public record Decision(boolean allowed, Duration retryAfter) {}
public interface RateLimiter { Decision allow(); }
// LeakyBucketRateLimiter, FixedWindowRateLimiter, ... implements RateLimiter
```

```ts
// TS (reasoning aid) — same shape
export interface RateLimiter { allow(): { allowed: boolean; retryAfterMs: number }; }
```

**Selection is config-driven at the composition root** (the *only* place that knows the concrete
type):

- **Go:** `app.Options` carries an algorithm name + params; a `newRateLimiter(cfg, clock)`
  factory `switch`es on the name and returns a `RateLimiter`. `NewRouter` builds it once and
  hands the interface to the middleware. New algorithm → new struct + one `case`.
- **Java:** an `@ConfigurationProperties` field (`app.ratelimit.algorithm`) + a `@Configuration`
  that produces the single `RateLimiter` bean via a factory (or `@ConditionalOnProperty` per
  impl). The filter constructor-injects the `RateLimiter` interface. New algorithm → new class +
  one factory branch.

**What this buys us live.** We ship **`LeakyBucket`** as the default and keep **`SlidingWindowLog`
pre-built** as the confirmed second strategy (below). If the interviewer wants strict semantics,
we enable that class — already unit-tested against the **same interface + same clock harness** —
and flip one config value; the middleware, filter, and wiring are untouched.

### 8.1 The second strategy: `SlidingWindowLog` (strict ≤10 per rolling 60s)

The one algorithm that delivers the *literal* reading of "10 requests / minute" — never more
than 10 in **any** rolling 60s window (unlike leaky/token/fixed-window, which allow ~20 at a
boundary; see §2 table).

- **State:** a queue of the timestamps of the last accepted requests.
- **On each request** (one atomic critical section, same as §6.2):
  1. Evict timestamps older than `now - window` (60s) from the front.
  2. If `count < limit (10)`: accept, append `now`.
  3. Else: reject 429; `Retry-After = window - (now - oldest timestamp)` (when the oldest entry
     ages out).
- **Trade-off vs leaky bucket:** exact guarantee, but **O(n) memory** — up to `limit` timestamps
  retained (here just 10, so trivial; for a *per-client* variant it's `limit × clients`). No
  smoothing/burst-shaping — it's a hard count, not a drip.
- **Same seam:** implements `RateLimiter.Allow()/allow()`, takes the injected clock, constructed
  by the same factory on `algorithm = "sliding_window_log"`.

### Red→green additions for the interface

- Insert **before §4.1:** define the `RateLimiter` interface + `Decision`; make the middleware
  depend on the interface. (`LeakyBucket` is the first implementation the §4.1–4.4 tests drive.)
- Add a later step: **`SlidingWindowLog`** implemented + tested against the *same* interface
  (allow 10 in a window / reject the 11th / oldest ages out → a slot frees / `Retry-After`
  correct), plus a **factory-selection test** (`algorithm` config picks the right impl) —
  proving OCP concretely and giving us the pre-built alternative to demo live.
