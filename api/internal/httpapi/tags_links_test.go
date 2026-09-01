package httpapi_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ipedrazas/idpad/api/internal/model"
)

// setTags replaces an idea's tag set and returns what the API stored.
func setTags(t *testing.T, srv *httptest.Server, ideaID string, names ...string) []model.Tag {
	t.Helper()
	var tags []model.Tag
	status := call(t, srv, http.MethodPut, "/api/v1/ideas/"+ideaID+"/tags",
		map[string]any{"tags": names}, &tags)
	require.Equal(t, http.StatusOK, status)
	return tags
}

func TestIdeaTagsReplaceWholeSet(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Tagged", doc("body"))

	tags := setTags(t, srv, idea.ID, "Machine Learning", "Go")
	require.Len(t, tags, 2)
	// Tags come back in slug order, which is what keeps the chips stable.
	require.Equal(t, []string{"go", "machine-learning"}, slugsOf(tags))
	require.Equal(t, "Machine Learning", tags[1].Name, "the typed name is preserved for display")

	// A second PUT is a replacement, not an addition.
	tags = setTags(t, srv, idea.ID, "Go", "Postgres")
	require.Equal(t, []string{"go", "postgres"}, slugsOf(tags))

	// An empty list is how the last tag is removed.
	tags = setTags(t, srv, idea.ID)
	require.Empty(t, tags)
}

func TestIdeaTagsFoldVariantsOntoOneTag(t *testing.T) {
	srv := newTestServer(t)
	first := createIdea(t, srv, "First", doc("a"))
	second := createIdea(t, srv, "Second", doc("b"))

	setTags(t, srv, first.ID, "Machine Learning")
	// A different spelling of the same concept must reuse the existing row
	// rather than creating a near-duplicate tag.
	setTags(t, srv, second.ID, "machine-learning")

	var tags []model.TagSummary
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/tags", nil, &tags))
	require.Len(t, tags, 1)
	require.Equal(t, "machine-learning", tags[0].Slug)
	require.Equal(t, 2, tags[0].IdeaCount)
}

func TestIdeaTagsDeduplicateWithinOneRequest(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Tagged", doc("body"))

	tags := setTags(t, srv, idea.ID, "AI", "ai", "  Ai  ")
	require.Len(t, tags, 1, "names folding onto one slug are a single tag")
	require.Equal(t, "ai", tags[0].Slug)
	require.Equal(t, "AI", tags[0].Name, "the first spelling seen wins the display name")
}

func TestIdeaTagsValidation(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Tagged", doc("body"))
	path := "/api/v1/ideas/" + idea.ID + "/tags"

	status, code := errorCode(t, srv, http.MethodPut, path, map[string]any{"tags": []string{"---"}})
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "validation_error", code)

	tooLong := make([]byte, model.MaxTagLen+1)
	for i := range tooLong {
		tooLong[i] = 'a'
	}
	status, _ = errorCode(t, srv, http.MethodPut, path, map[string]any{"tags": []string{string(tooLong)}})
	require.Equal(t, http.StatusBadRequest, status)

	many := make([]string, 26)
	for i := range many {
		many[i] = fmt.Sprintf("tag-%d", i)
	}
	status, _ = errorCode(t, srv, http.MethodPut, path, map[string]any{"tags": many})
	require.Equal(t, http.StatusBadRequest, status)

	// Blank entries are dropped rather than rejected: a chip editor emits them.
	tags := setTags(t, srv, idea.ID, "go", "   ", "")
	require.Equal(t, []string{"go"}, slugsOf(tags))
}

func TestIdeaTagsOnMissingIdea(t *testing.T) {
	srv := newTestServer(t)
	missing := "00000000-0000-7000-8000-000000000000"

	status, code := errorCode(t, srv, http.MethodGet, "/api/v1/ideas/"+missing+"/tags", nil)
	require.Equal(t, http.StatusNotFound, status)
	require.Equal(t, "not_found", code)

	status, _ = errorCode(t, srv, http.MethodPut, "/api/v1/ideas/"+missing+"/tags",
		map[string]any{"tags": []string{"go"}})
	require.Equal(t, http.StatusNotFound, status)
}

func TestListIdeasFilterByTag(t *testing.T) {
	srv := newTestServer(t)
	goIdea := createIdea(t, srv, "Go service", doc("a"))
	bothIdea := createIdea(t, srv, "Go and Postgres", doc("b"))
	createIdea(t, srv, "Untagged", doc("c"))

	setTags(t, srv, goIdea.ID, "Go")
	setTags(t, srv, bothIdea.ID, "Go", "Postgres")

	// One tag matches every idea carrying it.
	require.ElementsMatch(t,
		[]string{goIdea.ID, bothIdea.ID},
		idsOf(listIdeas(t, srv, url.Values{"tag": {"go"}})))

	// Stacking tags narrows: only ideas carrying all of them survive.
	require.Equal(t,
		[]string{bothIdea.ID},
		idsOf(listIdeas(t, srv, url.Values{"tag": {"go", "postgres"}})))

	// The filter is matched on the slug, so the display spelling also works.
	require.Equal(t,
		[]string{bothIdea.ID},
		idsOf(listIdeas(t, srv, url.Values{"tag": {"Postgres"}})))

	// No filter still lists everything.
	require.Len(t, listIdeas(t, srv, nil), 3)
}

func TestListIdeasCarriesTagsAndCounts(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Counted", doc("a"))
	other := createIdea(t, srv, "Other", doc("b"))
	setTags(t, srv, idea.ID, "Go")
	createLink(t, srv, idea.ID, other.ID, "references", nil)

	listed := listIdeas(t, srv, url.Values{"tag": {"go"}})
	require.Len(t, listed, 1)
	require.Equal(t, []string{"go"}, slugsOf(listed[0].Tags))
	require.Equal(t, 1, listed[0].LinkCount, "links count from either end")

	// The link's other end sees it in its own count too.
	all := listIdeas(t, srv, nil)
	for _, summary := range all {
		if summary.ID == other.ID {
			require.Equal(t, 1, summary.LinkCount)
			require.Empty(t, summary.Tags, "tags are never shared across ideas")
		}
	}
}

func TestListIdeasSearchAndExclude(t *testing.T) {
	srv := newTestServer(t)
	first := createIdea(t, srv, "Postgres indexes", doc("a"))
	createIdea(t, srv, "Go generics", doc("b"))

	// Search is a case-insensitive substring of the title.
	require.Equal(t, []string{first.ID}, idsOf(listIdeas(t, srv, url.Values{"q": {"postgres"}})))
	require.Equal(t, []string{first.ID}, idsOf(listIdeas(t, srv, url.Values{"q": {"INDEX"}})))
	require.Empty(t, listIdeas(t, srv, url.Values{"q": {"nothing here"}}))

	// exclude drops one idea, which is how the link picker avoids self-links.
	excluded := listIdeas(t, srv, url.Values{"exclude": {first.ID}})
	require.NotContains(t, idsOf(excluded), first.ID)

	status, code := errorCode(t, srv, http.MethodGet, "/api/v1/ideas?exclude=not-a-uuid", nil)
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "validation_error", code)
}

func TestDeleteUnusedTags(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Tagged", doc("a"))
	setTags(t, srv, idea.ID, "keep", "drop")
	setTags(t, srv, idea.ID, "keep")

	var result struct {
		Removed int `json:"removed"`
	}
	require.Equal(t, http.StatusOK,
		call(t, srv, http.MethodDelete, "/api/v1/tags/unused", nil, &result))
	require.Equal(t, 1, result.Removed)

	var tags []model.TagSummary
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/tags", nil, &tags))
	require.Equal(t, []string{"keep"}, slugsOfSummaries(tags))
}

func TestDeletingIdeaReleasesItsTags(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Doomed", doc("a"))
	setTags(t, srv, idea.ID, "orphan")

	require.Equal(t, http.StatusNoContent,
		call(t, srv, http.MethodDelete, "/api/v1/ideas/"+idea.ID, nil, nil))

	// The tag row survives the idea, but nothing carries it any more.
	var tags []model.TagSummary
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/tags", nil, &tags))
	require.Len(t, tags, 1)
	require.Equal(t, 0, tags[0].IdeaCount)
}

// createLink is the fixture the link tests start from.
func createLink(t *testing.T, srv *httptest.Server, sourceID, targetID, relation string, note *string) model.Link {
	t.Helper()
	payload := map[string]any{"target_idea_id": targetID, "relation": relation}
	if note != nil {
		payload["note"] = *note
	}
	var link model.Link
	status := call(t, srv, http.MethodPost, "/api/v1/ideas/"+sourceID+"/links", payload, &link)
	require.Equal(t, http.StatusCreated, status)
	return link
}

func listLinks(t *testing.T, srv *httptest.Server, ideaID string) []model.Link {
	t.Helper()
	var links []model.Link
	require.Equal(t, http.StatusOK,
		call(t, srv, http.MethodGet, "/api/v1/ideas/"+ideaID+"/links", nil, &links))
	return links
}

func TestLinkIsVisibleFromBothEnds(t *testing.T) {
	srv := newTestServer(t)
	source := createIdea(t, srv, "The claim", doc("a"))
	target := createIdea(t, srv, "The evidence", doc("b"))

	note := "backs up the second paragraph"
	link := createLink(t, srv, source.ID, target.ID, "references", &note)
	require.Equal(t, model.DirectionOutgoing, link.Direction)
	require.Equal(t, target.ID, link.OtherIdeaID)
	require.Equal(t, "The evidence", link.OtherTitle)
	require.Equal(t, note, *link.Note)

	// The source sees the link it stated, pointing outward.
	fromSource := listLinks(t, srv, source.ID)
	require.Len(t, fromSource, 1)
	require.Equal(t, model.DirectionOutgoing, fromSource[0].Direction)
	require.Equal(t, model.RelationReferences, fromSource[0].Relation)
	require.Equal(t, "The evidence", fromSource[0].OtherTitle)

	// The target learns it is pointed at, with the same stored relation and
	// the opposite direction; rendering the inverse is the client's job.
	fromTarget := listLinks(t, srv, target.ID)
	require.Len(t, fromTarget, 1)
	require.Equal(t, model.DirectionIncoming, fromTarget[0].Direction)
	require.Equal(t, model.RelationReferences, fromTarget[0].Relation)
	require.Equal(t, source.ID, fromTarget[0].OtherIdeaID)
	require.Equal(t, "The claim", fromTarget[0].OtherTitle)
	require.Equal(t, link.ID, fromTarget[0].ID, "both ends address the same row")
}

func TestLinkRelationsAreAllAccepted(t *testing.T) {
	srv := newTestServer(t)
	source := createIdea(t, srv, "Source", doc("a"))

	for _, relation := range model.Relations {
		target := createIdea(t, srv, "Target "+string(relation), doc("b"))
		link := createLink(t, srv, source.ID, target.ID, string(relation), nil)
		require.Equal(t, relation, link.Relation)
		require.Nil(t, link.Note)
	}
	require.Len(t, listLinks(t, srv, source.ID), len(model.Relations))
}

func TestLinkRejectsSelfAndDuplicates(t *testing.T) {
	srv := newTestServer(t)
	source := createIdea(t, srv, "Source", doc("a"))
	target := createIdea(t, srv, "Target", doc("b"))
	path := "/api/v1/ideas/" + source.ID + "/links"

	status, code := errorCode(t, srv, http.MethodPost, path,
		map[string]any{"target_idea_id": source.ID, "relation": "similar"})
	require.Equal(t, http.StatusUnprocessableEntity, status)
	require.Equal(t, "unprocessable", code)

	createLink(t, srv, source.ID, target.ID, "similar", nil)
	status, code = errorCode(t, srv, http.MethodPost, path,
		map[string]any{"target_idea_id": target.ID, "relation": "similar"})
	require.Equal(t, http.StatusConflict, status)
	require.Equal(t, "conflict", code)

	// A different relation between the same pair is a distinct statement.
	createLink(t, srv, source.ID, target.ID, "references", nil)
	require.Len(t, listLinks(t, srv, source.ID), 2)
}

func TestLinkValidation(t *testing.T) {
	srv := newTestServer(t)
	source := createIdea(t, srv, "Source", doc("a"))
	target := createIdea(t, srv, "Target", doc("b"))
	path := "/api/v1/ideas/" + source.ID + "/links"

	status, code := errorCode(t, srv, http.MethodPost, path,
		map[string]any{"target_idea_id": target.ID, "relation": "contradicts"})
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "validation_error", code)

	// An inverse is a rendering, not something the client may store.
	status, _ = errorCode(t, srv, http.MethodPost, path,
		map[string]any{"target_idea_id": target.ID, "relation": "referenced_by"})
	require.Equal(t, http.StatusBadRequest, status)

	status, _ = errorCode(t, srv, http.MethodPost, path,
		map[string]any{"target_idea_id": "nope", "relation": "similar"})
	require.Equal(t, http.StatusBadRequest, status)

	longNote := make([]byte, 281)
	for i := range longNote {
		longNote[i] = 'a'
	}
	status, _ = errorCode(t, srv, http.MethodPost, path,
		map[string]any{"target_idea_id": target.ID, "relation": "similar", "note": string(longNote)})
	require.Equal(t, http.StatusBadRequest, status)
}

func TestLinkToMissingIdea(t *testing.T) {
	srv := newTestServer(t)
	source := createIdea(t, srv, "Source", doc("a"))
	missing := "00000000-0000-7000-8000-000000000000"

	status, code := errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+source.ID+"/links",
		map[string]any{"target_idea_id": missing, "relation": "similar"})
	require.Equal(t, http.StatusNotFound, status)
	require.Equal(t, "not_found", code)

	// A missing source is a 404 too, before the target is ever considered.
	status, _ = errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+missing+"/links",
		map[string]any{"target_idea_id": source.ID, "relation": "similar"})
	require.Equal(t, http.StatusNotFound, status)
}

func TestDeleteLinkFromEitherEnd(t *testing.T) {
	srv := newTestServer(t)
	source := createIdea(t, srv, "Source", doc("a"))
	target := createIdea(t, srv, "Target", doc("b"))

	link := createLink(t, srv, source.ID, target.ID, "expands", nil)
	// The target did not create the link but may retract it.
	require.Equal(t, http.StatusNoContent,
		call(t, srv, http.MethodDelete, "/api/v1/links/"+link.ID, nil, nil))
	require.Empty(t, listLinks(t, srv, source.ID))
	require.Empty(t, listLinks(t, srv, target.ID))

	status, code := errorCode(t, srv, http.MethodDelete, "/api/v1/links/"+link.ID, nil)
	require.Equal(t, http.StatusNotFound, status)
	require.Equal(t, "not_found", code)
}

func TestDeletingIdeaRemovesItsLinks(t *testing.T) {
	srv := newTestServer(t)
	source := createIdea(t, srv, "Source", doc("a"))
	target := createIdea(t, srv, "Target", doc("b"))
	createLink(t, srv, source.ID, target.ID, "references", nil)

	// Deleting the target must clear the link the source declared at it,
	// otherwise the source would render a row pointing at nothing.
	require.Equal(t, http.StatusNoContent,
		call(t, srv, http.MethodDelete, "/api/v1/ideas/"+target.ID, nil, nil))
	require.Empty(t, listLinks(t, srv, source.ID))
}

func TestLinksOnMissingIdea(t *testing.T) {
	srv := newTestServer(t)
	status, code := errorCode(t, srv, http.MethodGet,
		"/api/v1/ideas/00000000-0000-7000-8000-000000000000/links", nil)
	require.Equal(t, http.StatusNotFound, status)
	require.Equal(t, "not_found", code)
}

// listIdeas fetches the idea list with the given query parameters.
func listIdeas(t *testing.T, srv *httptest.Server, query url.Values) []model.IdeaSummary {
	t.Helper()
	path := "/api/v1/ideas"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	var ideas []model.IdeaSummary
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, path, nil, &ideas))
	return ideas
}

func idsOf(ideas []model.IdeaSummary) []string {
	ids := make([]string, 0, len(ideas))
	for _, i := range ideas {
		ids = append(ids, i.ID)
	}
	return ids
}

func slugsOf(tags []model.Tag) []string {
	slugs := make([]string, 0, len(tags))
	for _, t := range tags {
		slugs = append(slugs, t.Slug)
	}
	return slugs
}

func slugsOfSummaries(tags []model.TagSummary) []string {
	slugs := make([]string, 0, len(tags))
	for _, t := range tags {
		slugs = append(slugs, t.Slug)
	}
	return slugs
}
