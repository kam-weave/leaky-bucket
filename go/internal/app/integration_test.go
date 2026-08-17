package app_test

// Integration tests: the rate limiter wired into the real router (app.NewRouter)
// with a real store, auth, and middleware, exercised over in-process HTTP. These
// verify cross-cutting behavior the ratelimit unit tests can't — one global bucket
// shared across different endpoints, headers on real endpoint responses, and the
// sliding-window strategy end-to-end through the router. See docs/testing.md.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/weave-lab/interview-public/go/internal/app"
	"github.com/weave-lab/interview-public/go/internal/ratelimit"
	"github.com/weave-lab/interview-public/go/internal/store"
)

// newServerWithConfig builds an isolated test server whose /api routes are guarded
// by a limiter built from cfg via the factory, driven by the returned clock.
func newServerWithConfig(t *testing.T, cfg ratelimit.Config) (*httptest.Server, *time.Time) {
	t.Helper()

	dir, err := os.MkdirTemp("", "ratelimit-integration-*")
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
	rl, err := ratelimit.New(cfg, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.NewRouter(s, app.Options{RateLimiter: rl}))
	t.Cleanup(srv.Close)
	return srv, &now
}

// One global bucket is shared across all /api endpoints: ten requests spread over
// different routes exhaust it, and the eleventh (on any route) is rejected.
func TestIntegration_OneBucketAcrossEndpoints(t *testing.T) {
	srv, _ := newServerWithConfig(t, ratelimit.Config{Limit: 10, Period: time.Minute})

	routes := []string{
		"/api/contacts", "/api/files", "/api/reports/activity",
		"/api/contacts", "/api/files", "/api/reports/activity",
		"/api/contacts", "/api/files", "/api/reports/activity",
		"/api/contacts", // 10th
	}
	for i, route := range routes {
		resp := authedGet(t, srv.URL+route)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("request %d (%s): unexpected 429 before the limit", i+1, route)
		}
	}

	// 11th request, on a different endpoint, is rejected by the shared bucket.
	over := authedGet(t, srv.URL+"/api/files")
	defer over.Body.Close()
	if over.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("11th request across endpoints: want 429, got %d", over.StatusCode)
	}
}

// RateLimit-* headers are present on successful (200) endpoint responses, and
// Remaining decreases as the budget is consumed.
func TestIntegration_HeadersOnSuccessfulResponses(t *testing.T) {
	srv, _ := newServerWithConfig(t, ratelimit.Config{Limit: 10, Period: time.Minute})

	first := authedGet(t, srv.URL+"/api/contacts")
	first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first request: want 200, got %d", first.StatusCode)
	}
	if got := first.Header.Get("RateLimit-Limit"); got != "10" {
		t.Fatalf("RateLimit-Limit: want 10, got %q", got)
	}
	if got := first.Header.Get("RateLimit-Remaining"); got != "9" {
		t.Fatalf("RateLimit-Remaining after 1st: want 9, got %q", got)
	}

	second := authedGet(t, srv.URL+"/api/contacts")
	second.Body.Close()
	if got := second.Header.Get("RateLimit-Remaining"); got != "8" {
		t.Fatalf("RateLimit-Remaining after 2nd: want 8, got %q", got)
	}
}

// The sliding-window strategy, selected by config, enforces a strict rolling cap
// end-to-end through the router: spaced requests fill the window, one just before
// the earliest ages out is rejected, and one frees after it ages out.
func TestIntegration_SlidingWindowStrictThroughRouter(t *testing.T) {
	srv, clk := newServerWithConfig(t, ratelimit.Config{
		Algorithm: ratelimit.AlgorithmSlidingWindowLog,
		Limit:     10,
		Period:    time.Minute,
	})
	start := *clk

	for i := 0; i < 10; i++ { // 10 requests spaced 1s apart, all within the window
		*clk = start.Add(time.Duration(i) * time.Second)
		resp := authedGet(t, srv.URL+"/api/contacts")
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("spaced request %d: unexpected 429", i+1)
		}
	}

	*clk = start.Add(59 * time.Second) // earliest still in the 60s window → 10 held
	blocked := authedGet(t, srv.URL+"/api/contacts")
	blocked.Body.Close()
	if blocked.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("at 59s: want 429, got %d", blocked.StatusCode)
	}

	*clk = start.Add(60*time.Second + time.Millisecond) // earliest ages out
	freed := authedGet(t, srv.URL+"/api/contacts")
	freed.Body.Close()
	if freed.StatusCode == http.StatusTooManyRequests {
		t.Fatal("after the earliest ages out: want a slot to free, got 429")
	}
}
