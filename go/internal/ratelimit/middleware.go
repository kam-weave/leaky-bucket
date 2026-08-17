package ratelimit

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/weave-lab/interview-public/go/internal/apierr"
)

// Middleware returns net/http middleware that admits requests through rl and
// rejects the rest with 429 Too Many Requests. On rejection it sets a Retry-After
// header (seconds, rounded up, at least 1) and a JSON body matching the API's
// error shape, and logs the rejection at warn level. Allowed requests are not
// logged (no per-request overhead). One rl instance is shared across all requests
// (the global limit). A nil logger falls back to slog.Default().
func Middleware(rl RateLimiter, logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			d := rl.Allow()
			if d.Allowed {
				next.ServeHTTP(w, r)
				return
			}

			secs := int(math.Ceil(d.RetryAfter.Seconds()))
			if secs < 1 {
				secs = 1
			}
			logger.Warn("rate limit exceeded",
				"method", r.Method, "path", r.URL.Path, "retry_after_s", secs)
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			apierr.Write(w, http.StatusTooManyRequests, "rate limit exceeded")
		})
	}
}
