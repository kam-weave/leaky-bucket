package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/weave-lab/interview-public/go/internal/app"
	"github.com/weave-lab/interview-public/go/internal/ratelimit"
	"github.com/weave-lab/interview-public/go/internal/store"
)

// newLimitedServer builds an isolated test server whose /api routes are guarded by
// a fresh leaky bucket. The returned *time.Time is the limiter's clock: advance it
// by reassigning (*clk = clk.Add(...)) to drive leaking deterministically. This
// deliberately does NOT reuse the shared main_test server, so limiter state can't
// leak across tests.
func newLimitedServer(t *testing.T, capacity int) (*httptest.Server, *time.Time) {
	t.Helper()

	dir, err := os.MkdirTemp("", "ratelimit-http-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	s, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	now := time.Now()
	lb := ratelimit.NewLeakyBucket(capacity, time.Minute, func() time.Time { return now })
	srv := httptest.NewServer(app.NewRouter(s, app.Options{RateLimiter: lb}))
	t.Cleanup(srv.Close)
	return srv, &now
}

func authedGet(t *testing.T, url string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer test@example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// T8 — 429 after burst: the 11th authenticated /api request in a burst is rejected
// with 429 and carries a Retry-After header.
func TestHTTP_429AfterBurst(t *testing.T) {
	srv, _ := newLimitedServer(t, 10)

	for i := 1; i <= 10; i++ {
		resp := authedGet(t, srv.URL+"/api/contacts")
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("request %d: unexpected 429", i)
		}
	}

	resp := authedGet(t, srv.URL+"/api/contacts")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("request 11: want 429, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Fatal("429 response missing Retry-After header")
	}
}

// T9 — /health is exempt: it lives outside /api, so far more than `capacity`
// health checks never trip the limit.
func TestHTTP_HealthExempt(t *testing.T) {
	srv, _ := newLimitedServer(t, 10)

	for i := 1; i <= 30; i++ {
		resp, err := http.Get(srv.URL + "/health")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("/health request %d: want 200, got %d", i, resp.StatusCode)
		}
	}
}

// T10 — unauthenticated requests count: because the limiter runs before auth, ten
// tokenless /api requests (each 401) still consume the bucket, so the 11th is
// rejected with 429 before auth even gets a chance to 401 it.
func TestHTTP_UnauthenticatedCounts(t *testing.T) {
	srv, _ := newLimitedServer(t, 10)

	for i := 1; i <= 10; i++ {
		resp, err := http.Get(srv.URL + "/api/contacts") // no Authorization header
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("unauth request %d: want 401, got %d", i, resp.StatusCode)
		}
	}

	resp, err := http.Get(srv.URL + "/api/contacts")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("unauth request 11: want 429 (bucket consumed by anon traffic), got %d", resp.StatusCode)
	}
}

// F4 — 429 contract + recovery over HTTP: the rejected response carries an exact
// Retry-After of "6" (one 6s drip) and a JSON {"error": ...} body, and after the
// clock advances one interval the limiter admits again.
func TestHTTP_RetryAfterValueBodyAndRecovery(t *testing.T) {
	srv, clk := newLimitedServer(t, 10)

	for i := 0; i < 10; i++ {
		authedGet(t, srv.URL+"/api/contacts").Body.Close()
	}

	resp := authedGet(t, srv.URL+"/api/contacts")
	if resp.StatusCode != http.StatusTooManyRequests {
		resp.Body.Close()
		t.Fatalf("11th request: want 429, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Retry-After"); got != "6" {
		resp.Body.Close()
		t.Fatalf("Retry-After: want \"6\", got %q", got)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		resp.Body.Close()
		t.Fatalf("decoding 429 body: %v", err)
	}
	resp.Body.Close()
	if body["error"] == "" {
		t.Fatalf("429 body: want a JSON error field, got %v", body)
	}

	*clk = clk.Add(6 * time.Second) // one leak interval frees a slot
	resp2 := authedGet(t, srv.URL+"/api/contacts")
	defer resp2.Body.Close()
	if resp2.StatusCode == http.StatusTooManyRequests {
		t.Fatal("after advancing 6s: want recovery, still got 429")
	}
}

// G2 — RateLimit-* headers: every /api response advertises the limit and remaining
// budget; a 429 also carries RateLimit-Reset.
func TestHTTP_RateLimitHeaders(t *testing.T) {
	srv, _ := newLimitedServer(t, 10)

	resp := authedGet(t, srv.URL+"/api/contacts") // request 1 of 10
	resp.Body.Close()
	if got := resp.Header.Get("RateLimit-Limit"); got != "10" {
		t.Fatalf("RateLimit-Limit: want \"10\", got %q", got)
	}
	if got := resp.Header.Get("RateLimit-Remaining"); got != "9" {
		t.Fatalf("RateLimit-Remaining after 1st: want \"9\", got %q", got)
	}

	for i := 0; i < 9; i++ { // exhaust the remaining budget (requests 2..10)
		authedGet(t, srv.URL+"/api/contacts").Body.Close()
	}

	over := authedGet(t, srv.URL+"/api/contacts") // request 11 → 429
	defer over.Body.Close()
	if over.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("11th request: want 429, got %d", over.StatusCode)
	}
	if got := over.Header.Get("RateLimit-Remaining"); got != "0" {
		t.Fatalf("RateLimit-Remaining on 429: want \"0\", got %q", got)
	}
	if over.Header.Get("RateLimit-Reset") == "" {
		t.Fatal("429 response missing RateLimit-Reset")
	}
}

// G3 — 401 is a JSON error envelope: unauthenticated /api requests now return the
// same {"error": ...} shape as the rest of the API (previously plain text).
func TestHTTP_UnauthorizedIsJSON(t *testing.T) {
	srv, _ := newLimitedServer(t, 10)

	resp, err := http.Get(srv.URL + "/api/contacts") // no Authorization header
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type: want application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding 401 body: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("401 body: want a JSON error field, got %v", body)
	}
}
