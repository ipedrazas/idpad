package httpapi

import (
	"net/http"
	"unicode/utf8"

	"github.com/ipedrazas/idpad/api/internal/model"
)

// maxTagsPerIdea keeps the chip row readable and bounds the work one write
// can ask the tag table to do.
const maxTagsPerIdea = 25

// setTagsRequest replaces an idea's whole tag set. Sending an empty list is
// how the last tag is removed, so the field is required rather than optional.
type setTagsRequest struct {
	Tags []string `json:"tags"`
}

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.store.ListTags(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

func (s *Server) handleListIdeaTags(w http.ResponseWriter, r *http.Request) {
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

	tags, err := s.store.ListTagsForIdea(r.Context(), ideaID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

func (s *Server) handleSetIdeaTags(w http.ResponseWriter, r *http.Request) {
	ideaID, err := pathUUID(r, "ideaID", "idea")
	if err != nil {
		writeError(w, r, err)
		return
	}

	var req setTagsRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}

	names, err := validateTagNames(req.Tags)
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

	tags, err := s.store.SetIdeaTags(r.Context(), ideaID, names)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

// handleDeleteUnusedTags prunes tags no idea carries. It is a maintenance
// endpoint: unused tags are harmless, but a vocabulary nobody cleans grows
// stale suggestions.
func (s *Server) handleDeleteUnusedTags(w http.ResponseWriter, r *http.Request) {
	removed, err := s.store.DeleteUnusedTags(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"removed": removed})
}

// validateTagNames trims, length-checks and de-duplicates a submitted tag set.
// De-duplication is by slug, so "AI" and "ai" collapse to one entry here
// rather than racing each other in the store.
func validateTagNames(raw []string) ([]string, error) {
	names := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))

	for _, candidate := range raw {
		name := model.NormaliseTagName(candidate)
		if name == "" {
			continue
		}
		if utf8.RuneCountInString(name) > model.MaxTagLen {
			return nil, validationError("tag %q must be at most %d characters", name, model.MaxTagLen)
		}
		slug := model.Slugify(name)
		if slug == "" {
			return nil, validationError("tag %q must contain at least one letter or digit", name)
		}
		if utf8.RuneCountInString(slug) > model.MaxTagLen {
			return nil, validationError("tag %q is too long once normalised", name)
		}
		if seen[slug] {
			continue
		}
		seen[slug] = true
		names = append(names, name)
	}

	if len(names) > maxTagsPerIdea {
		return nil, validationError("an idea can carry at most %d tags", maxTagsPerIdea)
	}
	return names, nil
}
