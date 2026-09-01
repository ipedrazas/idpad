package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ipedrazas/idpad/api/internal/model"
)

// setStatus moves an idea and returns it as the API stored it.
func setStatus(t *testing.T, srv *httptest.Server, ideaID string, status model.IdeaStatus) model.Idea {
	t.Helper()
	var idea model.Idea
	code := call(t, srv, http.MethodPatch, "/api/v1/ideas/"+ideaID+"/status",
		map[string]any{"status": string(status)}, &idea)
	require.Equal(t, http.StatusOK, code)
	return idea
}

func TestIdeaStartsAsDraft(t *testing.T) {
	srv := newTestServer(t)

	idea := createIdea(t, srv, "Fresh", doc("nothing decided yet"))
	require.Equal(t, model.IdeaDraft, idea.Status)
	// Nothing has moved yet, so the transition is as old as the idea itself.
	require.WithinDuration(t, idea.CreatedAt, idea.StatusChangedAt, 0)
}

func TestSetIdeaStatusMovesItAlong(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Ship it", doc("body"))

	moved := setStatus(t, srv, idea.ID, model.IdeaInProgress)
	require.Equal(t, model.IdeaInProgress, moved.Status)
	require.True(t, moved.StatusChangedAt.After(idea.StatusChangedAt))

	// The status survives a re-read rather than living only in the response.
	var fetched model.Idea
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID, nil, &fetched))
	require.Equal(t, model.IdeaInProgress, fetched.Status)
}

func TestSetIdeaStatusLeavesUpdatedAtAlone(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Untouched", doc("body"))

	moved := setStatus(t, srv, idea.ID, model.IdeaDone)
	// updated_at means "the body was edited", and marking an idea done edits
	// nothing it says — otherwise the list would reshuffle on every move.
	require.WithinDuration(t, idea.UpdatedAt, moved.UpdatedAt, 0)
}

func TestSetIdeaStatusToItselfDoesNotRefreshTheTimestamp(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Static", doc("body"))

	first := setStatus(t, srv, idea.ID, model.IdeaDone)
	again := setStatus(t, srv, idea.ID, model.IdeaDone)
	require.WithinDuration(t, first.StatusChangedAt, again.StatusChangedAt, 0,
		"re-setting the current status is a no-op, not a way to touch the timestamp")
}

func TestEditingAnIdeaLeavesItsStatusAlone(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "In flight", doc("body"))
	setStatus(t, srv, idea.ID, model.IdeaInProgress)

	// The editor's autosave sends only the title and body, so it cannot reset
	// a status it never showed.
	var updated model.Idea
	code := call(t, srv, http.MethodPut, "/api/v1/ideas/"+idea.ID,
		map[string]any{"title": "In flight, edited", "body": doc("rewritten")}, &updated)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, model.IdeaInProgress, updated.Status)
}

func TestSetIdeaStatusValidation(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Guarded", doc("body"))
	path := "/api/v1/ideas/" + idea.ID + "/status"

	code, errCode := errorCode(t, srv, http.MethodPatch, path, map[string]any{"status": "shipped"})
	require.Equal(t, http.StatusBadRequest, code)
	require.Equal(t, "validation_error", errCode)

	// Comment statuses are a different vocabulary and must not leak in here.
	code, _ = errorCode(t, srv, http.MethodPatch, path, map[string]any{"status": "resolved"})
	require.Equal(t, http.StatusBadRequest, code)

	code, errCode = errorCode(t, srv, http.MethodPatch,
		"/api/v1/ideas/00000000-0000-0000-0000-000000000000/status",
		map[string]any{"status": "done"})
	require.Equal(t, http.StatusNotFound, code)
	require.Equal(t, "not_found", errCode)
}

func TestListIdeasFilteredByStatus(t *testing.T) {
	srv := newTestServer(t)
	draft := createIdea(t, srv, "Draft one", doc("a"))
	working := createIdea(t, srv, "Working", doc("b"))
	done := createIdea(t, srv, "Finished", doc("c"))

	setStatus(t, srv, working.ID, model.IdeaInProgress)
	setStatus(t, srv, done.ID, model.IdeaDone)

	require.Equal(t, []string{working.ID}, listIdeaIDs(t, srv, "?status=in_progress"))

	// Statuses are ORed: an idea sits in exactly one, so asking for two can
	// only mean "either of these".
	either := listIdeaIDs(t, srv, "?status=draft&status=done")
	require.ElementsMatch(t, []string{draft.ID, done.ID}, either)

	// No status parameter still means every idea.
	require.Len(t, listIdeaIDs(t, srv, ""), 3)

	code, errCode := errorCode(t, srv, http.MethodGet, "/api/v1/ideas?status=shipped", nil)
	require.Equal(t, http.StatusBadRequest, code)
	require.Equal(t, "validation_error", errCode)
}

func TestListIdeasCombinesStatusAndTagFilters(t *testing.T) {
	srv := newTestServer(t)
	wanted := createIdea(t, srv, "Tagged and working", doc("a"))
	otherStatus := createIdea(t, srv, "Tagged but draft", doc("b"))
	otherTag := createIdea(t, srv, "Working but untagged", doc("c"))

	setTags(t, srv, wanted.ID, "Go")
	setTags(t, srv, otherStatus.ID, "Go")
	setStatus(t, srv, wanted.ID, model.IdeaInProgress)
	setStatus(t, srv, otherTag.ID, model.IdeaInProgress)

	// The two filters are ANDed with each other, so each one narrows.
	require.Equal(t, []string{wanted.ID}, listIdeaIDs(t, srv, "?tag=go&status=in_progress"))
}

func TestStatusIndexReportsEveryStatus(t *testing.T) {
	srv := newTestServer(t)
	first := createIdea(t, srv, "One", doc("a"))
	createIdea(t, srv, "Two", doc("b"))
	setStatus(t, srv, first.ID, model.IdeaDone)

	var counts []model.IdeaStatusSummary
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/statuses", nil, &counts))

	// Every status is reported in lifecycle order, empty ones included, so a
	// filter chip never appears or vanishes as ideas move.
	require.Equal(t, model.IdeaStatuses, statusesOf(counts))
	byStatus := map[model.IdeaStatus]int{}
	for _, c := range counts {
		byStatus[c.Status] = c.IdeaCount
	}
	require.Equal(t, map[model.IdeaStatus]int{
		model.IdeaDraft: 1, model.IdeaInProgress: 0, model.IdeaDone: 1, model.IdeaRejected: 0,
	}, byStatus)
}

// listIdeaIDs returns the ids the list endpoint yields for a query string.
func listIdeaIDs(t *testing.T, srv *httptest.Server, query string) []string {
	t.Helper()
	var ideas []model.IdeaSummary
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/ideas"+query, nil, &ideas))

	ids := make([]string, 0, len(ideas))
	for _, idea := range ideas {
		ids = append(ids, idea.ID)
	}
	return ids
}

func statusesOf(counts []model.IdeaStatusSummary) []model.IdeaStatus {
	statuses := make([]model.IdeaStatus, 0, len(counts))
	for _, c := range counts {
		statuses = append(statuses, c.Status)
	}
	return statuses
}
