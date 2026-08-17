package app_test

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
