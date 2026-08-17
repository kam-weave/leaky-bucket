# CRM API Server

A simple CRM-style API server for managing contacts and files. Available in both Go and Java implementations with identical API interfaces.

## Implementations

- **Go**: Located in `go/` directory
- **Java**: Located in `java/` directory (Spring Boot)

Both implementations share the same SQLite database schema and provide identical REST API endpoints.

## How to review this repo

The **rate limiter** is the feature to review. It was built AI-assisted but with a deliberate,
auditable process — **design first, then test-first (red→green), one small commit per step** —
so there's more written material than a typical change. That's intentional: the docs are the
reasoning, and the git history is the receipt. You don't need to read everything; here's the
short path.

**Recommended reading order (≈15 min):**

1. **[`docs/rate-limiter-plan.md`](docs/rate-limiter-plan.md)** — the design. What we built and
   *why*: the leaky-bucket algorithm, the exact "10/min" guarantee (and its honest caveats), the
   `RateLimiter` interface, and where it plugs into the request pipeline. Start here.
2. **[`docs/architecture-go.md`](docs/architecture-go.md)** — how the existing Go server is wired
   and the one seam the limiter hooks into. (Diagrams render inline on GitHub.)
3. **The code** — small and self-contained under
   [`go/internal/ratelimit/`](go/internal/ratelimit/): `ratelimit.go` (the interface),
   `leakybucket.go`, `slidingwindow.go`, `middleware.go`, `factory.go`, and the tests in
   `ratelimit_test.go`; plus the wiring in `go/internal/app/app.go` and the HTTP tests in
   `go/internal/app/app_test.go`.
4. **[`docs/TASKS.md`](docs/TASKS.md)** — the build log: every test we wrote, the change that
   made it pass, and the findings from an adversarial code-review pass (and how each was fixed).
   Read this to see *how* the code came to exist, commit by commit.

**Where to find what:**

| Looking for… | Go there |
|--------------|----------|
| Why it's designed this way | `docs/rate-limiter-plan.md` |
| The limiter code | `go/internal/ratelimit/` |
| How it's wired into the API | `go/internal/app/app.go` (the `/api` middleware) |
| The tests | `go/internal/ratelimit/ratelimit_test.go`, `go/internal/app/app_test.go` |
| The commit-by-commit story + review findings | `docs/TASKS.md` |
| Tradeoffs made & future work | `docs/design-tradeoffs.md` |
| Onboarding for an AI assistant | `CLAUDE.md` |

**A note on the process:** commits are grouped one-per-task and tell the story (`T1…T15` = the
feature, `F1…F5` = adversarial-review fixes, `G1…G3` = consistency cleanups). Reading the
history top-to-bottom shows each test appearing with the code that satisfies it.

**Diagrams** are inline [Mermaid](https://mermaid.js.org/) — they render directly on GitHub. To
view them locally, run **`make diagrams`** (renders to SVG, opens a browser gallery; needs
Node/`npx`, no editor extension). Optional VS Code inline preview: `make setup` installs the
`bierner.markdown-mermaid` extension (`Cmd+Shift+V`); if it renders blank under a dark theme, use
`make diagrams`.

## Quick Start

### Go

```bash
cd go
make seed   # Seed the database with test data
make run    # Start the server
```

Or without make:

```bash
cd go
go run ./cmd/server --seed
go run ./cmd/server
```

### Java

```bash
cd java
make seed   # Seed the database with test data
make run    # Start the server
```

Or without make:

```bash
cd java
./mvnw spring-boot:run -Dspring-boot.run.arguments="--app.seed=true"
./mvnw spring-boot:run
```

The server runs on `http://localhost:8080` by default.

## Authentication

All `/api/*` endpoints require a bearer token in the `Authorization` header:

```bash
curl -H "Authorization: Bearer user@example.com" http://localhost:8080/api/contacts
```

The token should be a valid email address. It serves as the user identifier.

## Rate Limiting (Go)

The Go server enforces a **process-local, global rate limit**: **10 requests per minute across
all clients combined**, applied to `/api/**`. `/health` is exempt.

- **Algorithm:** a leaky bucket (counter form) — allows a burst of up to 10, then refills one
  slot every ~6s. This is *smoothing*, not a strict rolling window: the worst case across a
  poorly-aligned 60s window is ~20. A strict **sliding-window log** (never >10 in any rolling
  60s) is also included and selectable by config.
- **Placement:** the limiter is the front door of `/api` — it runs **before** auth, so
  unauthenticated requests count too (a 429 preempts the 401).
- **On rejection:** HTTP **429** with a JSON `{"error": ...}` body and a `Retry-After` header.
  Every `/api` response also carries `RateLimit-Limit` / `RateLimit-Remaining` (and
  `RateLimit-Reset` on a 429).

Configured in `go/cmd/server/main.go` via the factory:

```go
ratelimit.New(ratelimit.Config{
    Algorithm: ratelimit.AlgorithmLeakyBucket, // or AlgorithmSlidingWindowLog
    Limit:     10,
    Period:    time.Minute,
}, time.Now)
```

Swapping the strategy is a one-line `Algorithm` change — the middleware and wiring depend only
on the `ratelimit.RateLimiter` interface (Open/Closed). See
[`docs/rate-limiter-plan.md`](docs/rate-limiter-plan.md) for the full design and
[`docs/TASKS.md`](docs/TASKS.md) for the test-by-test build log.

```bash
# Watch the limit trip: the 11th call within a minute returns 429.
for i in $(seq 1 11); do
  curl -s -o /dev/null -w "%{http_code}\n" \
    -H "Authorization: Bearer user@example.com" http://localhost:8080/api/contacts
done
```

## Endpoints

### Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check (no auth required) |

### Contacts

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/contacts` | List contacts (paginated) |
| `GET` | `/api/contacts/:id` | Get a contact |
| `POST` | `/api/contacts` | Create a contact |
| `PUT` | `/api/contacts/:id` | Update a contact |
| `DELETE` | `/api/contacts/:id` | Delete a contact |
| `POST` | `/api/contacts/import` | Bulk import contacts (JSON array, max 10k) |
| `GET` | `/api/contacts/export` | Export all contacts as CSV |

### Files

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/files` | List files |
| `POST` | `/api/files` | Upload a file (multipart form, max 100MB) |
| `GET` | `/api/files/:id` | Download a file |

### Reports

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/reports/activity` | Activity report (last 30 days by default) |

## Pagination

The `/api/contacts` endpoint uses cursor-based pagination. The response includes a `next_page_token` field when more results are available:

```json
{
  "contacts": [...],
  "next_page_token": "eyJjcmVhdGVkX2F0Ijoi..."
}
```

Pass the token as a query parameter to fetch the next page:

```bash
curl -H "Authorization: Bearer user@example.com" \
  "http://localhost:8080/api/contacts?page_token=eyJjcmVhdGVkX2F0Ijoi..."
```

## Examples

```bash
# List contacts (first page)
curl -H "Authorization: Bearer user@example.com" \
  http://localhost:8080/api/contacts?limit=10

# Create a contact
curl -X POST -H "Authorization: Bearer user@example.com" \
  -H "Content-Type: application/json" \
  -d '{"first_name":"Jane","last_name":"Doe","email":"jane@example.com","phone":"555-1234","company":"Acme"}' \
  http://localhost:8080/api/contacts

# Upload a file
curl -X POST -H "Authorization: Bearer user@example.com" \
  -F "file=@/path/to/file.pdf" \
  http://localhost:8080/api/files

# Bulk import
curl -X POST -H "Authorization: Bearer user@example.com" \
  -H "Content-Type: application/json" \
  -d '[{"first_name":"A","last_name":"B","email":"a@b.com","phone":"555","company":"X"}]' \
  http://localhost:8080/api/contacts/import

# Export contacts
curl -H "Authorization: Bearer user@example.com" \
  http://localhost:8080/api/contacts/export > contacts.csv
```

## Seeding Options

### Go

```bash
cd go
make seed                                      # Default: 10k contacts, 20 files
go run ./cmd/server --seed --contacts=50000    # Custom contact count
go run ./cmd/server --seed --files=100         # Custom file count
```

### Java

```bash
cd java
make seed                                                                              # Default: 10k contacts, 20 files
./mvnw spring-boot:run -Dspring-boot.run.arguments="--app.seed=true --contacts=50000"  # Custom contact count
```

To reset the database:

```bash
cd go && make reset   # Go
cd java && make reset # Java
```

## Benchmarks (Go only)

```bash
cd go
make bench
```

## Configuration

### Go

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `:8080` | Server listen address |
| `--data` | `data` | Data directory for database and files |
| `--seed` | `false` | Seed database with test data |
| `--contacts` | `10000` | Number of contacts to seed |
| `--files` | `20` | Number of files to seed |

### Java

Configuration is in `src/main/resources/application.properties`:

| Property | Default | Description |
|----------|---------|-------------|
| `server.port` | `8080` | Server listen port |
| `app.data-dir` | `data` | Data directory for database and files |
| `app.max-upload-size` | `104857600` | Max file upload size (100MB) |
