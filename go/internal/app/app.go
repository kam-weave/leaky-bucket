package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/weave-lab/interview-public/go/internal/api"
	"github.com/weave-lab/interview-public/go/internal/auth"
	"github.com/weave-lab/interview-public/go/internal/ratelimit"
	"github.com/weave-lab/interview-public/go/internal/store"
)

type Options struct {
	EnableLogging bool
	// RateLimiter, when set, guards all /api routes with a global rate limit.
	// It is applied before auth (so unauthenticated requests still count) and
	// does not cover /health. Nil disables rate limiting (e.g. in benchmarks).
	RateLimiter ratelimit.RateLimiter
}

func NewRouter(s *store.Store, opts Options) http.Handler {
	a := api.New(s)
	r := chi.NewRouter()

	if opts.EnableLogging {
		r.Use(middleware.Logger)
	}
	r.Use(middleware.Recoverer)
	r.Use(auth.TokenParser)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		// Rate limit before auth: the limiter is the front door of /api, so
		// unauthenticated requests count and /health (outside /api) is exempt.
		if opts.RateLimiter != nil {
			r.Use(ratelimit.Middleware(opts.RateLimiter))
		}
		r.Use(auth.RequireAuth)

		r.Post("/auth/token", a.HandleToken)

		r.Get("/contacts", a.HandleListContacts)
		r.Post("/contacts", a.HandleCreateContact)
		r.Get("/contacts/{id}", a.HandleGetContact)
		r.Put("/contacts/{id}", a.HandleUpdateContact)
		r.Delete("/contacts/{id}", a.HandleDeleteContact)

		r.Post("/contacts/import", a.HandleImportContacts)
		r.Get("/contacts/export", a.HandleExportContacts)

		r.Get("/files", a.HandleListFiles)
		r.Post("/files", a.HandleUploadFile)
		r.Get("/files/{id}", a.HandleDownloadFile)

		r.Get("/reports/activity", a.HandleActivityReport)
	})

	return r
}
