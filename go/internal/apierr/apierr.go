// Package apierr writes the API's standard JSON error envelope so every layer
// (handlers, auth, rate limiter) emits the same {"error": ...} shape.
package apierr

import (
	"encoding/json"
	"net/http"
)

// Write sends status with a JSON body {"error": msg} and the JSON content type.
// Set any additional headers (Retry-After, RateLimit-*, ...) before calling it.
func Write(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
