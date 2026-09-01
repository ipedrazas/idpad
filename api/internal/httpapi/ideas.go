package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

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

// updateIdeaRequest is the PUT /ideas/{id} payload: a full replacement.
type updateIdeaRequest struct {
	Title string          `json:"title"`
	Body  json.RawMessage `json:"body"`
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

func (s *Server) handleListIdeas(w http.ResponseWriter, r *http.Request) {
	ideas, err := s.store.ListIdeas(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ideas)
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
