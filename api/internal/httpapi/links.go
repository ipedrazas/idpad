package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ipedrazas/idpad/api/internal/model"
	"github.com/ipedrazas/idpad/api/internal/store"
)

// maxLinkNoteLen mirrors the varchar(280) column: a note says why the link
// exists, not what the other idea contains.
const maxLinkNoteLen = 280

// createLinkRequest connects the idea in the path to another one. The idea in
// the path is always the source, so the relation reads in the direction the
// user stated it.
type createLinkRequest struct {
	TargetIdeaID string  `json:"target_idea_id"`
	Relation     string  `json:"relation"`
	Note         *string `json:"note"`
}

func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request) {
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

	links, err := s.store.ListLinksForIdea(r.Context(), ideaID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, links)
}

func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	sourceID, err := pathUUID(r, "ideaID", "idea")
	if err != nil {
		writeError(w, r, err)
		return
	}

	var req createLinkRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}

	relation := model.Relation(strings.TrimSpace(req.Relation))
	if !relation.Valid() {
		writeError(w, r, validationError("relation must be one of %s", joinRelations()))
		return
	}

	targetID, err := uuid.Parse(strings.TrimSpace(req.TargetIdeaID))
	if err != nil {
		writeError(w, r, validationError("target_idea_id %q is not a valid UUID", req.TargetIdeaID))
		return
	}
	// Caught here as well as by the check constraint, so the user gets a
	// message about the link rather than a database error code.
	if targetID.String() == sourceID {
		writeError(w, r, unprocessableError("an idea cannot be linked to itself"))
		return
	}

	note, err := validateLinkNote(req.Note)
	if err != nil {
		writeError(w, r, err)
		return
	}

	exists, err := s.store.IdeaExists(r.Context(), sourceID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !exists {
		writeError(w, r, notFoundError("idea"))
		return
	}

	link, err := s.store.CreateLink(r.Context(), sourceID, targetID.String(), relation, note)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, r, conflictError("these ideas are already connected by that relation"))
		return
	}
	// The source was just confirmed to exist, so a missing row can only be the
	// target the user picked.
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, notFoundError("target idea"))
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

// handleDeleteLink removes a link from either end: a link states something
// about a pair of ideas, so both sides can retract it.
func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "linkID", "link")
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.store.DeleteLink(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, notFoundError("link"))
			return
		}
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validateLinkNote trims the optional note, treating an empty one as absent.
func validateLinkNote(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	note := strings.TrimSpace(*raw)
	if note == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(note) > maxLinkNoteLen {
		return nil, validationError("note must be at most %d characters", maxLinkNoteLen)
	}
	return &note, nil
}

// joinRelations renders the accepted relations for an error message.
func joinRelations() string {
	names := make([]string, 0, len(model.Relations))
	for _, r := range model.Relations {
		names = append(names, string(r))
	}
	return strings.Join(names, ", ")
}
