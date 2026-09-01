package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/ipedrazas/idpad/api/internal/config"
	"github.com/ipedrazas/idpad/api/internal/store"
	"github.com/ipedrazas/idpad/api/internal/tagger"
)

// Server wires the store and configuration into the HTTP handlers. It holds
// every dependency explicitly; the package has no package-level state.
type Server struct {
	store  *store.Store
	log    *slog.Logger
	cfg    config.Config
	tagger *tagger.Client
}

// NewServer builds the server and returns it ready to be mounted. The tagging
// client is nil when no service is configured, which the routes treat as the
// feature being switched off rather than as an error.
func NewServer(st *store.Store, log *slog.Logger, cfg config.Config) *Server {
	return &Server{
		store:  st,
		log:    log,
		cfg:    cfg,
		tagger: tagger.New(cfg.TaggerURL, cfg.TaggerTimeout),
	}
}

// Router returns the fully configured HTTP handler for the service.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	// chi's RealIP is deliberately not used: it trusts client-settable headers
	// (GHSA-3fxj-6jh8-hvhx), and nothing here needs the caller's address.
	r.Use(requestLogger(s.log))
	r.Use(middleware.Recoverer)
	r.Use(cors(s.cfg))
	// A zero timeout would hand every handler an already-expired context, so
	// treat it as "no deadline" rather than "no time at all".
	if s.cfg.RequestTimeout > 0 {
		r.Use(middleware.Timeout(s.cfg.RequestTimeout))
	}

	r.Get("/healthz", s.handleHealth)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/ideas", func(r chi.Router) {
			r.Post("/", s.handleCreateIdea)
			r.Get("/", s.handleListIdeas)

			r.Route("/{ideaID}", func(r chi.Router) {
				r.Get("/", s.handleGetIdea)
				r.Put("/", s.handleUpdateIdea)
				r.Delete("/", s.handleDeleteIdea)

				r.Get("/comments", s.handleListComments)
				r.Post("/comments", s.handleCreateComment)

				r.Get("/resources", s.handleListResources)
				r.Post("/resources", s.handleCreateResource)

				r.Get("/tags", s.handleListIdeaTags)
				// PUT, not POST: the client sends the tag set it wants the
				// idea to end up with, which is what a chip editor produces.
				r.Put("/tags", s.handleSetIdeaTags)
				r.Post("/tags/auto", s.handleAutoTagIdea)

				r.Get("/links", s.handleListLinks)
				r.Post("/links", s.handleCreateLink)
			})
		})

		r.Route("/comments/{commentID}", func(r chi.Router) {
			r.Put("/", s.handleUpdateComment)
			r.Delete("/", s.handleDeleteComment)
			r.Patch("/status", s.handleSetCommentStatus)
		})

		r.Delete("/resources/{resourceID}", s.handleDeleteResource)

		r.Route("/tags", func(r chi.Router) {
			r.Get("/", s.handleListTags)
			r.Delete("/unused", s.handleDeleteUnusedTags)
		})

		// Lets the UI hide affordances the deployment cannot serve, rather
		// than offering a button that always fails.
		r.Get("/features", s.handleFeatures)

		r.Delete("/links/{linkID}", s.handleDeleteLink)
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, apiError{Status: http.StatusNotFound, Code: codeNotFound, Message: "no such endpoint"})
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, apiError{
			Status:  http.StatusMethodNotAllowed,
			Code:    codeBadRequest,
			Message: "method not allowed for this endpoint",
		})
	})

	return r
}

// handleHealth reports process and database liveness.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		loggerFrom(r.Context()).Error("health check failed", "error", err)
		writeError(w, r, apiError{
			Status:  http.StatusServiceUnavailable,
			Code:    codeInternal,
			Message: "database unreachable",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleFeatures reports which optional capabilities this deployment has.
func (s *Server) handleFeatures(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"auto_tagging": s.tagger.Enabled(),
	})
}

// pathUUID reads a UUID path parameter, rejecting malformed ids with a 400 so
// they are never confused with a well-formed id that simply does not exist.
func pathUUID(r *http.Request, param, what string) (string, error) {
	raw := chi.URLParam(r, param)
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", validationError("%s id %q is not a valid UUID", what, raw)
	}
	return id.String(), nil
}
