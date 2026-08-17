package api

import (
	"encoding/json"
	"net/http"

	"github.com/weave-lab/interview-public/go/internal/apierr"
	"github.com/weave-lab/interview-public/go/internal/store"
)

type API struct {
	store *store.Store
}

func New(s *store.Store) *API {
	return &API{store: s}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	apierr.Write(w, status, msg)
}
