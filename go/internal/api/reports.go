package api

import (
	"net/http"
	"time"

	"github.com/weave-lab/interview-public/go/internal/auth"
)

func (a *API) HandleActivityReport(w http.ResponseWriter, r *http.Request) {
	since := time.Now().AddDate(0, 0, -30) // last 30 days

	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			since = t
		}
	}

	report, err := a.store.GenerateActivityReport(r.Context(), since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate report")
		return
	}

	a.store.LogActivity(r.Context(), auth.UserID(r.Context()), "view", "report", "activity")
	writeJSON(w, http.StatusOK, report)
}
