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

// maxCommentLen keeps a single comment to a sane size.
const maxCommentLen = 10_000

// anchorPayload is the wire shape of a comment anchor.
type anchorPayload struct {
	BlockIndex  int    `json:"block_index"`
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
	Snippet     string `json:"snippet"`
}

// createCommentRequest starts a thread (anchor set) or adds a reply
// (parent_id set). The two are mutually exclusive.
type createCommentRequest struct {
	ParentID *string        `json:"parent_id"`
	Anchor   *anchorPayload `json:"anchor"`
	Body     string         `json:"body"`
}

// updateCommentRequest edits the text of a comment or reply.
type updateCommentRequest struct {
	Body string `json:"body"`
}

// setStatusRequest opens or resolves a thread.
type setStatusRequest struct {
	Status string `json:"status"`
}

// handleListComments returns the idea's threads, each with its replies and a
// detached flag derived from the current body text.
func (s *Server) handleListComments(w http.ResponseWriter, r *http.Request) {
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

	comments, err := s.store.ListCommentsForIdea(r.Context(), ideaID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	blocks, err := model.BlockTexts(idea.Body)
	if err != nil {
		// A body that cannot be flattened can no longer support any anchor,
		// so every thread is reported as detached rather than failing the read.
		blocks = nil
	}

	writeJSON(w, http.StatusOK, buildThreads(comments, blocks))
}

// buildThreads assembles flat comment rows into thread-shaped JSON. Roots keep
// the order the store returned (newest first) and replies are appended oldest
// first. A reply whose root is missing is dropped, which cannot happen while
// the parent_id cascade holds but keeps the assembly total.
func buildThreads(comments []model.Comment, blocks []string) []model.Thread {
	threads := make([]model.Thread, 0, len(comments))
	index := make(map[string]int, len(comments))

	for _, c := range comments {
		if c.ParentID != nil {
			continue
		}
		detached := c.Anchor == nil || !c.Anchor.Matches(blocks)
		index[c.ID] = len(threads)
		threads = append(threads, model.Thread{
			Comment:  c,
			Detached: detached,
			Replies:  []model.Comment{},
		})
	}

	for _, c := range comments {
		if c.ParentID == nil {
			continue
		}
		if pos, ok := index[*c.ParentID]; ok {
			threads[pos].Replies = append(threads[pos].Replies, c)
		}
	}

	return threads
}

func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	ideaID, err := pathUUID(r, "ideaID", "idea")
	if err != nil {
		writeError(w, r, err)
		return
	}

	var req createCommentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}

	body, err := validateCommentBody(req.Body)
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

	if req.ParentID != nil {
		s.createReply(w, r, ideaID, *req.ParentID, body, req.Anchor)
		return
	}

	if req.Anchor == nil {
		writeError(w, r, validationError("a thread comment requires an anchor"))
		return
	}

	blocks, err := model.BlockTexts(idea.Body)
	if err != nil {
		writeError(w, r, unprocessableError("the idea body cannot be anchored to: %v", err))
		return
	}

	anchor := model.Anchor{
		BlockIndex:  req.Anchor.BlockIndex,
		StartOffset: req.Anchor.StartOffset,
		EndOffset:   req.Anchor.EndOffset,
		Snippet:     req.Anchor.Snippet,
	}
	if err := anchor.Validate(blocks); err != nil {
		writeError(w, r, unprocessableError("%v", err))
		return
	}

	comment, err := s.store.CreateThread(r.Context(), ideaID, anchor, body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

// createReply validates that the parent is a thread root on the same idea and
// that the reply carries no anchor of its own.
func (s *Server) createReply(w http.ResponseWriter, r *http.Request, ideaID, rawParentID, body string, anchor *anchorPayload) {
	if anchor != nil {
		writeError(w, r, unprocessableError("a reply must not carry an anchor"))
		return
	}

	parentID, parseErr := uuid.Parse(rawParentID)
	if parseErr != nil {
		writeError(w, r, validationError("parent_id %q is not a valid UUID", rawParentID))
		return
	}

	parent, err := s.store.GetComment(r.Context(), parentID.String())
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, notFoundError("parent comment"))
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	if parent.IdeaID != ideaID {
		writeError(w, r, unprocessableError("parent comment belongs to a different idea"))
		return
	}
	if parent.ParentID != nil {
		writeError(w, r, unprocessableError("replies are only allowed on a thread root, not on another reply"))
		return
	}

	comment, err := s.store.CreateReply(r.Context(), ideaID, parent.ID, body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

func (s *Server) handleUpdateComment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "commentID", "comment")
	if err != nil {
		writeError(w, r, err)
		return
	}

	var req updateCommentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}

	body, err := validateCommentBody(req.Body)
	if err != nil {
		writeError(w, r, err)
		return
	}

	comment, err := s.store.UpdateCommentBody(r.Context(), id, body)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, notFoundError("comment"))
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, comment)
}

func (s *Server) handleSetCommentStatus(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "commentID", "comment")
	if err != nil {
		writeError(w, r, err)
		return
	}

	var req setStatusRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}

	status := model.CommentStatus(req.Status)
	if !status.Valid() {
		writeError(w, r, validationError("status must be %q or %q", model.StatusOpen, model.StatusResolved))
		return
	}

	comment, err := s.store.SetCommentStatus(r.Context(), id, status)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, notFoundError("comment"))
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, comment)
}

func (s *Server) handleDeleteComment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "commentID", "comment")
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.store.DeleteComment(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, notFoundError("comment"))
			return
		}
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validateCommentBody trims and length-checks comment text.
func validateCommentBody(raw string) (string, error) {
	body := strings.TrimSpace(raw)
	if body == "" {
		return "", validationError("comment body is required")
	}
	if utf8.RuneCountInString(body) > maxCommentLen {
		return "", validationError("comment body must be at most %d characters", maxCommentLen)
	}
	return body, nil
}
