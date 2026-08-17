# CLAUDE.md — orientation for AI assistants & new devs

A CRM API server implemented **twice** with identical REST APIs: Go (`go/`, chi router) and
Java Spring Boot (`java/`). Shared SQLite schema. Auth is a bearer token that is literally an
email (`Authorization: Bearer user@example.com`). `/api/**` requires auth; `/health` is public.

## Read these first (don't re-derive from source)

| Doc | What it gives you |
|-----|-------------------|
| [`docs/TASKS.md`](docs/TASKS.md) | Running task log + interview talking points. **Append one line per commit.** |
| [`docs/rate-limiter-plan.md`](docs/rate-limiter-plan.md) | Current work: Leaky Bucket design + red→green plan (has a Mermaid diagram). |
| [`docs/architecture-go.md`](docs/architecture-go.md) | Go request pipeline; the `r.Use(...)` seam in `go/internal/app/app.go`; test setup. |
| [`docs/architecture-java.md`](docs/architecture-java.md) | Java Filter/Interceptor pipeline; the `@Order(2)` filter seam; MockMvc test setup. |

## Current task

Implement a **process-local, global Leaky Bucket rate limiter: 10 requests/minute**, in
**both** stacks. `/health` is **exempt** (limiter applies to `/api/**`). See the plan doc.

## How we work here

- **Test-first, red→green.** Write a failing test → commit → make it pass → commit. Small,
  single-purpose commits telling the story.
- **Deterministic tests** — inject a clock into the limiter; no real `sleep`/`Thread.sleep`.
- **Keep docs tight; avoid markdown sprawl.** Update the existing docs above rather than adding
  new files. Diagrams are inline Mermaid (renders on GitHub) — not separate image files.

## Run / test

- **View diagrams:** `make diagrams` — renders the inline Mermaid in `docs/` to SVG and opens a
  browser gallery (reliable, extension-free). They also render on GitHub automatically.
  `make setup` installs an optional VS Code inline-preview extension, but it can render blank
  under dark themes — prefer `make diagrams`.
- Go: `cd go && make seed && make run` · `make test`
- Java: `cd java && make seed && make run` · `make test`
