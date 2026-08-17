package ratelimit

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// F5 — the middleware logs rejections (at warn) but not allowed requests.
func TestMiddleware_LogsRejectionOnly(t *testing.T) {
	now := time.Now()
	lb := NewLeakyBucket(1, time.Minute, fixedClock(&now)) // capacity 1: 1 allowed, then reject

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	h := Middleware(lb, logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request is allowed → no log.
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/x", nil))
	if buf.Len() != 0 {
		t.Fatalf("allowed request should not log, got: %s", buf.String())
	}

	// Second request is rejected → exactly one warn line.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/x", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", rec.Code)
	}
	logged := buf.String()
	if !strings.Contains(logged, "rate limit exceeded") || !strings.Contains(logged, "path=/api/x") {
		t.Fatalf("want a rejection log with the path, got: %q", logged)
	}
	if strings.Count(logged, "\n") != 1 {
		t.Fatalf("want exactly one log line, got: %q", logged)
	}
}
