package ratelimit

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
)

// Middleware returns net/http middleware that admits requests through rl and
// rejects the rest with 429 Too Many Requests. On rejection it sets a Retry-After
// header (seconds, rounded up, at least 1) and a JSON body matching the API's
// error shape. One rl instance is shared across all requests (the global limit).
func Middleware(rl RateLimiter) func(http.Handler) http.Handler {
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
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
		})
	}
}
