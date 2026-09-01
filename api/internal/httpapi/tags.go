package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/ipedrazas/idpad/api/internal/model"
	"github.com/ipedrazas/idpad/api/internal/store"
	"github.com/ipedrazas/idpad/api/internal/tagger"
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

// handleAutoTagIdea sends the idea's title and body to the tagging service and
// merges what comes back into the idea's existing tags.
//
// Merging rather than replacing is deliberate: a suggestion should never
// silently discard tags someone chose by hand, and an unwanted suggestion is
// one click to remove.
func (s *Server) handleAutoTagIdea(w http.ResponseWriter, r *http.Request) {
	if !s.tagger.Enabled() {
		writeError(w, r, apiError{
			Status:  http.StatusServiceUnavailable,
			Code:    codeUnavailable,
			Message: "automatic tagging is not configured on this server",
		})
		return
	}

	ideaID, err := pathUUID(r, "ideaID", "idea")
	if err != nil {
		writeError(w, r, err)
		return
	}

	idea, err := s.store.GetIdea(r.Context(), ideaID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, notFoundError("idea"))
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}

	text := ideaText(idea)
	if text == "" {
		writeError(w, r, unprocessableError("there is nothing to tag yet — write the idea first"))
		return
	}

	// The tagging service is markedly slower on a cold start than when warm,
	// past the global request timeout, so the call runs on its own budget.
	// Detaching also means a caller that navigates away still gets the tags
	// written, which is the useful outcome rather than wasted work.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), s.cfg.TaggerTimeout)
	defer cancel()

	suggested, err := s.tagger.Tag(ctx, text)
	if errors.Is(err, tagger.ErrUnavailable) {
		// Logged with the cause; the client is told only that it failed, so
		// the upstream's status and body are never echoed back.
		loggerFrom(r.Context()).Warn("tagging service call failed", "error", err, "idea_id", ideaID)
		writeError(w, r, apiError{
			Status:  http.StatusBadGateway,
			Code:    codeUpstream,
			Message: "the tagging service could not be reached, please try again",
		})
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}

	existing, err := s.store.ListTagsForIdea(r.Context(), ideaID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	names, err := validateTagNames(mergeTagNames(existing, suggested))
	if err != nil {
		writeError(w, r, err)
		return
	}

	tags, err := s.store.SetIdeaTags(r.Context(), ideaID, names)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

// ideaText flattens an idea into the plain text the tagging service reads.
// The title leads because it is the most concentrated description of the idea.
func ideaText(idea model.Idea) string {
	parts := []string{idea.Title}
	if blocks, err := model.BlockTexts(idea.Body); err == nil {
		parts = append(parts, blocks...)
	}

	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	return strings.Join(kept, "\n")
}

// mergeTagNames appends the suggestions the idea does not already carry.
// Existing names come first so their spelling is the one that survives
// de-duplication, and so a suggestion can never displace a hand-picked tag.
//
// Each suggestion is judged on its own: the service is not bound by this API's
// tag rules, so one unusable suggestion is skipped rather than costing the
// good ones alongside it. The result is therefore always something
// validateTagNames accepts.
func mergeTagNames(existing []model.Tag, suggested []string) []string {
	merged := tagNamesOf(existing)

	seen := make(map[string]bool, len(existing))
	for _, tag := range existing {
		seen[tag.Slug] = true
	}

	for _, raw := range suggested {
		if len(merged) >= maxTagsPerIdea {
			break
		}
		name := model.NormaliseTagName(raw)
		if !usableTagName(name) {
			continue
		}
		slug := model.Slugify(name)
		if seen[slug] {
			continue
		}
		seen[slug] = true
		merged = append(merged, name)
	}
	return merged
}

// usableTagName reports whether a suggested name is one this API can store,
// mirroring the per-name checks in validateTagNames.
func usableTagName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > model.MaxTagLen {
		return false
	}
	slug := model.Slugify(name)
	return slug != "" && utf8.RuneCountInString(slug) <= model.MaxTagLen
}

// tagNamesOf projects tags onto their display names.
func tagNamesOf(tags []model.Tag) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names
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
