package api

import (
	"net/http"

	"github.com/weave-lab/interview-public/go/internal/auth"
)

func (a *API) HandleToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "invalid or missing token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"user_id": userID,
		"status":  "authenticated",
	})
}
