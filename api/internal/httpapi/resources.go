package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/ipedrazas/idpad/api/internal/model"
	"github.com/ipedrazas/idpad/api/internal/store"
)

// maxLabelLen bounds the optional human-readable label.
const maxLabelLen = 200

// createResourceRequest is the POST /ideas/{id}/resources payload.
type createResourceRequest struct {
	Type  string  `json:"type"`
	URL   string  `json:"url"`
	Label *string `json:"label"`
}

func (s *Server) handleListResources(w http.ResponseWriter, r *http.Request) {
	ideaID, err := pathUUID(r, "ideaID", "idea")
	if err != nil {
		writeError(w, r, err)
		return
	}

	exists, err := s.store.IdeaExists(r.Context(), ideaID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !exists {
		writeError(w, r, notFoundError("idea"))
		return
	}

	resources, err := s.store.ListResourcesForIdea(r.Context(), ideaID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resources)
}

func (s *Server) handleCreateResource(w http.ResponseWriter, r *http.Request) {
	ideaID, err := pathUUID(r, "ideaID", "idea")
	if err != nil {
		writeError(w, r, err)
		return
	}

	var req createResourceRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}

	kind := model.ResourceType(strings.TrimSpace(req.Type))
	if !kind.Valid() {
		writeError(w, r, validationError("type must be %q or %q", model.ResourceImage, model.ResourceLink))
		return
	}

	resourceURL, err := validateResourceURL(req.URL)
	if err != nil {
		writeError(w, r, err)
		return
	}

	label, err := validateLabel(req.Label)
	if err != nil {
		writeError(w, r, err)
		return
	}

	exists, err := s.store.IdeaExists(r.Context(), ideaID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !exists {
		writeError(w, r, notFoundError("idea"))
		return
	}

	resource, err := s.store.CreateResource(r.Context(), ideaID, kind, resourceURL, label)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, r, conflictError("this URL is already attached to the idea"))
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, resource)
}

func (s *Server) handleDeleteResource(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "resourceID", "resource")
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.store.DeleteResource(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, notFoundError("resource"))
			return
		}
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validateResourceURL requires an absolute http(s) URL, which is what both the
// thumbnail and the link card can actually render.
func validateResourceURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", validationError("url is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", validationError("url is not a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", validationError("url must start with http:// or https://")
	}
	if parsed.Host == "" {
		return "", validationError("url must include a host")
	}
	return trimmed, nil
}

// validateLabel trims the optional label, treating an empty one as absent.
func validateLabel(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	label := strings.TrimSpace(*raw)
	if label == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(label) > maxLabelLen {
		return nil, validationError("label must be at most %d characters", maxLabelLen)
	}
	return &label, nil
}
