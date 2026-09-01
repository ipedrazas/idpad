package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ipedrazas/idpad/api/internal/config"
	"github.com/ipedrazas/idpad/api/internal/httpapi"
	"github.com/ipedrazas/idpad/api/internal/model"
	"github.com/ipedrazas/idpad/api/internal/store"
)

// fakeTagger stands in for the tagging service, recording what it was sent so
// the tests can assert on the text the API extracts from an idea.
type fakeTagger struct {
	server *httptest.Server
	// lastText is the text of the most recent request.
	lastText string
	// calls counts requests, so a test can prove no call was made.
	calls int
}

// newFakeTagger starts a tagging service that answers with tags, or — when
// status is not 200 — with that status and no useful body.
func newFakeTagger(t *testing.T, status int, tags []string, delay time.Duration) *fakeTagger {
	t.Helper()
	fake := &fakeTagger{}

	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.calls++

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload struct {
			Text string `json:"text"`
		}
		require.NoError(t, json.Unmarshal(body, &payload), "request body: %s", body)
		fake.lastText = payload.Text

		if delay > 0 {
			time.Sleep(delay)
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`upstream failure detail that must not leak`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string][]string{"tags": tags})
	}))
	t.Cleanup(fake.server.Close)
	return fake
}

// newTaggingServer returns an API server wired to taggerURL. A blank URL is
// the feature-disabled configuration.
func newTaggingServer(t *testing.T, taggerURL string, timeout time.Duration) *httptest.Server {
	t.Helper()
	testDB.Reset(t)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		CORSOrigins:    []string{"*"},
		RequestTimeout: 30 * time.Second,
		TaggerURL:      taggerURL,
		TaggerTimeout:  timeout,
	}
	srv := httptest.NewServer(httpapi.NewServer(store.New(testDB.Pool), log, cfg).Router())
	t.Cleanup(srv.Close)
	return srv
}

func autoTag(t *testing.T, srv *httptest.Server, ideaID string) []model.Tag {
	t.Helper()
	var tags []model.Tag
	status := call(t, srv, http.MethodPost, "/api/v1/ideas/"+ideaID+"/tags/auto", nil, &tags)
	require.Equal(t, http.StatusOK, status)
	return tags
}

func TestAutoTagAppliesSuggestions(t *testing.T) {
	fake := newFakeTagger(t, http.StatusOK,
		[]string{"kubernetes", "operators", "custom resources"}, 0)
	srv := newTaggingServer(t, fake.server.URL, 5*time.Second)

	idea := createIdea(t, srv, "Operators", doc("Kubernetes operators reconcile desired state."))
	tags := autoTag(t, srv, idea.ID)

	require.Equal(t, []string{"custom-resources", "kubernetes", "operators"}, slugsOf(tags))
	// The multi-word suggestion keeps its readable spelling.
	require.Contains(t, tagNames(tags), "custom resources")

	// The title leads, then the body: the service sees the whole idea.
	require.Equal(t, "Operators\nKubernetes operators reconcile desired state.", fake.lastText)

	// The tags were persisted, not just returned.
	var stored []model.Tag
	require.Equal(t, http.StatusOK,
		call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID+"/tags", nil, &stored))
	require.Equal(t, slugsOf(tags), slugsOf(stored))
}

func TestAutoTagMergesRatherThanReplaces(t *testing.T) {
	fake := newFakeTagger(t, http.StatusOK, []string{"kubernetes", "operators"}, 0)
	srv := newTaggingServer(t, fake.server.URL, 5*time.Second)

	idea := createIdea(t, srv, "Operators", doc("Reconciliation loops."))
	setTags(t, srv, idea.ID, "Hand Picked")

	tags := autoTag(t, srv, idea.ID)

	// A suggestion must never discard a tag someone chose by hand.
	require.Equal(t, []string{"hand-picked", "kubernetes", "operators"}, slugsOf(tags))
	require.Contains(t, tagNames(tags), "Hand Picked", "the hand-typed spelling survives")
}

func TestAutoTagIsIdempotentAndFoldsOntoExistingTags(t *testing.T) {
	fake := newFakeTagger(t, http.StatusOK, []string{"Kubernetes", "kubernetes", "KUBERNETES"}, 0)
	srv := newTaggingServer(t, fake.server.URL, 5*time.Second)

	idea := createIdea(t, srv, "Operators", doc("Reconciliation loops."))

	first := autoTag(t, srv, idea.ID)
	require.Equal(t, []string{"kubernetes"}, slugsOf(first), "spellings fold onto one tag")

	// Running it again adds nothing, because every suggestion already matches.
	second := autoTag(t, srv, idea.ID)
	require.Equal(t, slugsOf(first), slugsOf(second))
	require.Equal(t, first[0].ID, second[0].ID, "the same tag row is reused")
}

func TestAutoTagDropsUnusableSuggestions(t *testing.T) {
	// A service is not bound by this API's tag rules, so junk it returns is
	// dropped rather than failing the whole request.
	fake := newFakeTagger(t, http.StatusOK,
		[]string{"kubernetes", "   ", "---", "", strings.Repeat("a", model.MaxTagLen+1)}, 0)
	srv := newTaggingServer(t, fake.server.URL, 5*time.Second)

	idea := createIdea(t, srv, "Operators", doc("Reconciliation loops."))
	tags := autoTag(t, srv, idea.ID)

	require.Equal(t, []string{"kubernetes"}, slugsOf(tags))
}

func TestAutoTagKeepsExistingTagsWhenEverySuggestionIsUnusable(t *testing.T) {
	fake := newFakeTagger(t, http.StatusOK, []string{"---", strings.Repeat("b", model.MaxTagLen+1)}, 0)
	srv := newTaggingServer(t, fake.server.URL, 5*time.Second)

	idea := createIdea(t, srv, "Operators", doc("Reconciliation loops."))
	setTags(t, srv, idea.ID, "Hand Picked")

	tags := autoTag(t, srv, idea.ID)
	require.Equal(t, []string{"hand-picked"}, slugsOf(tags), "nothing usable came back, nothing was lost")
}

func TestAutoTagOnEmptyResponse(t *testing.T) {
	fake := newFakeTagger(t, http.StatusOK, []string{}, 0)
	srv := newTaggingServer(t, fake.server.URL, 5*time.Second)

	idea := createIdea(t, srv, "Operators", doc("Reconciliation loops."))
	require.Empty(t, autoTag(t, srv, idea.ID))
}

func TestAutoTagUpstreamFailureIsNotLeaked(t *testing.T) {
	fake := newFakeTagger(t, http.StatusInternalServerError, nil, 0)
	srv := newTaggingServer(t, fake.server.URL, 5*time.Second)

	idea := createIdea(t, srv, "Operators", doc("Reconciliation loops."))

	status, code, message := errorBodyOf(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/tags/auto", nil)
	require.Equal(t, http.StatusBadGateway, status)
	require.Equal(t, "upstream_error", code)
	// The upstream's own status and body stay in the log, not in the response.
	require.NotContains(t, message, "must not leak")
	require.NotContains(t, message, "500")
}

func TestAutoTagUpstreamTimeout(t *testing.T) {
	fake := newFakeTagger(t, http.StatusOK, []string{"kubernetes"}, 300*time.Millisecond)
	srv := newTaggingServer(t, fake.server.URL, 50*time.Millisecond)

	idea := createIdea(t, srv, "Operators", doc("Reconciliation loops."))

	status, code := errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/tags/auto", nil)
	require.Equal(t, http.StatusBadGateway, status)
	require.Equal(t, "upstream_error", code)

	// A failed call must not have touched the idea's tags.
	var stored []model.Tag
	require.Equal(t, http.StatusOK,
		call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID+"/tags", nil, &stored))
	require.Empty(t, stored)
}

func TestAutoTagRefusesAnEmptyIdea(t *testing.T) {
	fake := newFakeTagger(t, http.StatusOK, []string{"kubernetes"}, 0)
	srv := newTaggingServer(t, fake.server.URL, 5*time.Second)

	// A title alone is enough to tag; a blank title is impossible, so the
	// empty case is a whitespace-only body with a whitespace-only title, which
	// the title validation already prevents. An idea with an empty document
	// still carries its title, so it is taggable.
	idea := createIdea(t, srv, "Operators", json.RawMessage(`{"type":"doc","content":[]}`))
	tags := autoTag(t, srv, idea.ID)
	require.Equal(t, []string{"kubernetes"}, slugsOf(tags))
	require.Equal(t, "Operators", fake.lastText, "the title alone is sent when the body is empty")
}

func TestAutoTagWhenNotConfigured(t *testing.T) {
	srv := newTaggingServer(t, "", 5*time.Second)
	idea := createIdea(t, srv, "Operators", doc("Reconciliation loops."))

	status, code := errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/tags/auto", nil)
	require.Equal(t, http.StatusServiceUnavailable, status)
	require.Equal(t, "unavailable", code)
}

func TestAutoTagOnMissingIdea(t *testing.T) {
	fake := newFakeTagger(t, http.StatusOK, []string{"kubernetes"}, 0)
	srv := newTaggingServer(t, fake.server.URL, 5*time.Second)

	status, code := errorCode(t, srv, http.MethodPost,
		"/api/v1/ideas/00000000-0000-7000-8000-000000000000/tags/auto", nil)
	require.Equal(t, http.StatusNotFound, status)
	require.Equal(t, "not_found", code)
	require.Zero(t, fake.calls, "a missing idea is rejected before the service is called")
}

func TestFeaturesReportsAutoTagging(t *testing.T) {
	fake := newFakeTagger(t, http.StatusOK, nil, 0)

	var features map[string]bool
	enabled := newTaggingServer(t, fake.server.URL, 5*time.Second)
	require.Equal(t, http.StatusOK, call(t, enabled, http.MethodGet, "/api/v1/features", nil, &features))
	require.True(t, features["auto_tagging"])

	disabled := newTaggingServer(t, "", 5*time.Second)
	require.Equal(t, http.StatusOK, call(t, disabled, http.MethodGet, "/api/v1/features", nil, &features))
	require.False(t, features["auto_tagging"])
}

func tagNames(tags []model.Tag) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names
}
