package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ipedrazas/idpad/api/internal/model"
	"github.com/ipedrazas/idpad/api/internal/store"
)

// maxTitleLen mirrors the varchar(200) column.
const maxTitleLen = 200

// createIdeaRequest is the POST /ideas payload. Body is optional; omitting it
// creates an idea with an empty TipTap document.
type createIdeaRequest struct {
	Title string          `json:"title"`
	Body  json.RawMessage `json:"body"`
}

// updateIdeaRequest is the PUT /ideas/{id} payload: a full replacement of the
// title and body. Status is not part of it, so an autosaving editor can never
// reset a status it was not showing.
type updateIdeaRequest struct {
	Title string          `json:"title"`
	Body  json.RawMessage `json:"body"`
}

// setIdeaStatusRequest is the PATCH /ideas/{id}/status payload.
type setIdeaStatusRequest struct {
	Status string `json:"status"`
}

func (s *Server) handleCreateIdea(w http.ResponseWriter, r *http.Request) {
	var req createIdeaRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}

	title, err := validateTitle(req.Title)
	if err != nil {
		writeError(w, r, err)
		return
	}

	body := req.Body
	if len(body) == 0 || string(body) == "null" {
		body = model.EmptyDoc()
	}
	if err := validateBody(body); err != nil {
		writeError(w, r, err)
		return
	}

	idea, err := s.store.CreateIdea(r.Context(), title, body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, idea)
}

// handleListIdeas lists ideas, narrowed by the optional query parameters:
//
//	?tag=a&tag=b        only ideas carrying every listed tag
//	?status=a&status=b  only ideas in one of the listed states
//	?q=text             case-insensitive title substring
//	?exclude=uuid       drop one idea, so the link picker never offers self-linking
func (s *Server) handleListIdeas(w http.ResponseWriter, r *http.Request) {
	filter, err := parseIdeaFilter(r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	ideas, err := s.store.ListIdeas(r.Context(), filter)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ideas)
}

// parseIdeaFilter reads the list query parameters. Tag values are slugified so
// ?tag=Machine%20Learning finds the same ideas as ?tag=machine-learning.
func parseIdeaFilter(r *http.Request) (model.IdeaFilter, error) {
	query := r.URL.Query()

	var filter model.IdeaFilter
	for _, raw := range query["tag"] {
		slug := model.Slugify(raw)
		if slug == "" {
			return model.IdeaFilter{}, validationError("tag %q is not a usable tag name", raw)
		}
		filter.TagSlugs = append(filter.TagSlugs, slug)
	}

	// Tags are ANDed and statuses ORed, because an idea carries many tags but
	// sits in exactly one state: asking for two states can only mean "either".
	for _, raw := range query["status"] {
		status := model.IdeaStatus(strings.TrimSpace(raw))
		if !status.Valid() {
			return model.IdeaFilter{}, validationError("status %q is not a known status", raw)
		}
		filter.Statuses = append(filter.Statuses, status)
	}

	filter.Query = strings.TrimSpace(query.Get("q"))
	if utf8.RuneCountInString(filter.Query) > maxTitleLen {
		return model.IdeaFilter{}, validationError("q must be at most %d characters", maxTitleLen)
	}

	if raw := strings.TrimSpace(query.Get("exclude")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return model.IdeaFilter{}, validationError("exclude %q is not a valid UUID", raw)
		}
		filter.ExcludeID = id.String()
	}

	return filter, nil
}

func (s *Server) handleGetIdea(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "ideaID", "idea")
	if err != nil {
		writeError(w, r, err)
		return
	}

	idea, err := s.store.GetIdea(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, notFoundError("idea"))
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, idea)
}

func (s *Server) handleUpdateIdea(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "ideaID", "idea")
	if err != nil {
		writeError(w, r, err)
		return
	}

	var req updateIdeaRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}

	title, err := validateTitle(req.Title)
	if err != nil {
		writeError(w, r, err)
		return
	}

	body := req.Body
	if len(body) == 0 || string(body) == "null" {
		body = model.EmptyDoc()
	}
	if err := validateBody(body); err != nil {
		writeError(w, r, err)
		return
	}

	idea, err := s.store.UpdateIdea(r.Context(), id, title, body)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, notFoundError("idea"))
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, idea)
}

// handleSetIdeaStatus moves one idea along its lifecycle. It is a PATCH of its
// own rather than a field on the PUT so that saving the body and changing the
// status stay independent: neither write can clobber the other.
func (s *Server) handleSetIdeaStatus(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "ideaID", "idea")
	if err != nil {
		writeError(w, r, err)
		return
	}

	var req setIdeaStatusRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}

	status := model.IdeaStatus(strings.TrimSpace(req.Status))
	if !status.Valid() {
		writeError(w, r, validationError("status must be one of %s", quotedStatuses()))
		return
	}

	idea, err := s.store.SetIdeaStatus(r.Context(), id, status)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, notFoundError("idea"))
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, idea)
}

// handleListIdeaStatuses reports every status with how many ideas are in it,
// which is what the filter chips render. Counts ignore the current filter, as
// the tag index's do.
func (s *Server) handleListIdeaStatuses(w http.ResponseWriter, r *http.Request) {
	counts, err := s.store.CountIdeasByStatus(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, counts)
}

// quotedStatuses renders the accepted statuses for an error message.
func quotedStatuses() string {
	quoted := make([]string, 0, len(model.IdeaStatuses))
	for _, status := range model.IdeaStatuses {
		quoted = append(quoted, strconv.Quote(string(status)))
	}
	return strings.Join(quoted, ", ")
}

func (s *Server) handleDeleteIdea(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "ideaID", "idea")
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.store.DeleteIdea(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, notFoundError("idea"))
			return
		}
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validateTitle trims and length-checks an idea title.
func validateTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if title == "" {
		return "", validationError("title is required")
	}
	if utf8.RuneCountInString(title) > maxTitleLen {
		return "", validationError("title must be at most %d characters", maxTitleLen)
	}
	return title, nil
}

// validateBody checks that a body is a TipTap document. The JSON itself is
// stored verbatim so nothing outside this check is normalised away.
func validateBody(body json.RawMessage) error {
	if _, err := model.BlockTexts(body); err != nil {
		return validationError("body must be a TipTap document object with type \"doc\"")
	}
	return nil
}
