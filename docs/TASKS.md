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

## Phase 2 — Implementation (not started)

**Decisions confirmed:** implement in **both Go and Java**; **`/health` is exempt** (limiter
applies to `/api/**` only). Next: go red→green, one commit per step, starting with the core
limiter unit tests against an injected clock.

_To be filled in as we go, one line per commit._
