package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ipedrazas/idpad/api/internal/config"
	"github.com/ipedrazas/idpad/api/internal/httpapi"
	"github.com/ipedrazas/idpad/api/internal/model"
	"github.com/ipedrazas/idpad/api/internal/store"
	"github.com/ipedrazas/idpad/api/internal/testutil"
)

// testDB is the package-wide container, started once in TestMain.
var testDB *testutil.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	db, cleanup, err := testutil.NewPostgres(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration tests need a working Docker daemon: %v\n", err)
		os.Exit(1)
	}
	testDB = db

	code := m.Run()
	cleanup()
	os.Exit(code)
}

// newTestServer returns an httptest server wrapping the real router against a
// freshly truncated database.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	testDB.Reset(t)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{CORSOrigins: []string{"*"}, RequestTimeout: 0}
	srv := httptest.NewServer(httpapi.NewServer(store.New(testDB.Pool), log, cfg).Router())
	t.Cleanup(srv.Close)
	return srv
}

// doc builds a single-paragraph TipTap document.
func doc(paragraphs ...string) json.RawMessage {
	content := make([]map[string]any, 0, len(paragraphs))
	for _, p := range paragraphs {
		content = append(content, map[string]any{
			"type":    "paragraph",
			"content": []map[string]any{{"type": "text", "text": p}},
		})
	}
	body, err := json.Marshal(map[string]any{"type": "doc", "content": content})
	if err != nil {
		panic(err)
	}
	return body
}

// call performs a request and decodes the {"data": ...} envelope into out.
func call(t *testing.T, srv *httptest.Server, method, path string, payload any, out any) int {
	t.Helper()

	var reader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path, reader)
	require.NoError(t, err)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, res.Body.Close()) }()

	raw, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	if out != nil && res.StatusCode >= 200 && res.StatusCode < 300 && len(raw) > 0 {
		var env struct {
			Data json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(raw, &env), "response body: %s", raw)
		require.NoError(t, json.Unmarshal(env.Data, out), "data: %s", env.Data)
	}
	return res.StatusCode
}

// errorCode decodes the error envelope's code field.
func errorCode(t *testing.T, srv *httptest.Server, method, path string, payload any) (int, string) {
	t.Helper()

	var reader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path, reader)
	require.NoError(t, err)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, res.Body.Close()) }()

	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	raw, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &env), "response body: %s", raw)
	return res.StatusCode, env.Error.Code
}

// createIdea is the fixture most tests start from.
func createIdea(t *testing.T, srv *httptest.Server, title string, body json.RawMessage) model.Idea {
	t.Helper()
	var idea model.Idea
	status := call(t, srv, http.MethodPost, "/api/v1/ideas",
		map[string]any{"title": title, "body": body}, &idea)
	require.Equal(t, http.StatusCreated, status)
	return idea
}

func TestIdeaCRUD(t *testing.T) {
	srv := newTestServer(t)

	idea := createIdea(t, srv, "  Build idpad  ", doc("We should ship this."))
	require.Equal(t, "Build idpad", idea.Title, "title should be trimmed")
	require.NotEmpty(t, idea.ID)
	require.False(t, idea.CreatedAt.IsZero())

	var fetched model.Idea
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID, nil, &fetched))
	require.Equal(t, idea.ID, fetched.ID)
	require.JSONEq(t, string(doc("We should ship this.")), string(fetched.Body))

	var updated model.Idea
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodPut, "/api/v1/ideas/"+idea.ID,
		map[string]any{"title": "Build idpad v2", "body": doc("Rewritten.")}, &updated))
	require.Equal(t, "Build idpad v2", updated.Title)
	require.True(t, updated.UpdatedAt.After(idea.UpdatedAt), "updated_at should advance on update")

	var list []model.IdeaSummary
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/ideas", nil, &list))
	require.Len(t, list, 1)
	require.Equal(t, 0, list[0].CommentCount)
	require.Equal(t, 0, list[0].ResourceCount)

	require.Equal(t, http.StatusNoContent, call(t, srv, http.MethodDelete, "/api/v1/ideas/"+idea.ID, nil, nil))
	require.Equal(t, http.StatusNotFound, call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID, nil, nil))
}

func TestIdeaValidation(t *testing.T) {
	srv := newTestServer(t)

	status, code := errorCode(t, srv, http.MethodPost, "/api/v1/ideas", map[string]any{"title": "   "})
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "validation_error", code)

	status, _ = errorCode(t, srv, http.MethodPost, "/api/v1/ideas",
		map[string]any{"title": string(bytes.Repeat([]byte("a"), 201))})
	require.Equal(t, http.StatusBadRequest, status)

	status, _ = errorCode(t, srv, http.MethodPost, "/api/v1/ideas",
		map[string]any{"title": "ok", "body": map[string]any{"type": "paragraph"}})
	require.Equal(t, http.StatusBadRequest, status, "body must be a doc node")

	status, _ = errorCode(t, srv, http.MethodGet, "/api/v1/ideas/not-a-uuid", nil)
	require.Equal(t, http.StatusBadRequest, status)

	status, _ = errorCode(t, srv, http.MethodGet, "/api/v1/ideas/018f0000-0000-7000-8000-000000000000", nil)
	require.Equal(t, http.StatusNotFound, status)

	// A body-less create is allowed and yields an empty document.
	var idea model.Idea
	require.Equal(t, http.StatusCreated,
		call(t, srv, http.MethodPost, "/api/v1/ideas", map[string]any{"title": "no body"}, &idea))
	require.JSONEq(t, `{"type":"doc","content":[]}`, string(idea.Body))
}

func TestCommentThreadLifecycle(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Anchored", doc("We should definitely ship this."))

	var thread model.Comment
	status := call(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments", map[string]any{
		"body": "Do we have the capacity?",
		"anchor": map[string]any{
			"block_index": 0, "start_offset": 10, "end_offset": 20, "snippet": "definitely",
		},
	}, &thread)
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, thread.ParentID)
	require.NotNil(t, thread.Anchor)
	require.Equal(t, "definitely", thread.Anchor.Snippet)
	require.Equal(t, model.StatusOpen, thread.Status)

	var reply model.Comment
	status = call(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments", map[string]any{
		"parent_id": thread.ID, "body": "Yes, next sprint.",
	}, &reply)
	require.Equal(t, http.StatusCreated, status)
	require.Equal(t, thread.ID, *reply.ParentID)
	require.Nil(t, reply.Anchor)

	var threads []model.Thread
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID+"/comments", nil, &threads))
	require.Len(t, threads, 1)
	require.False(t, threads[0].Detached)
	require.Len(t, threads[0].Replies, 1)
	require.Equal(t, reply.ID, threads[0].Replies[0].ID)

	var edited model.Comment
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodPut, "/api/v1/comments/"+thread.ID,
		map[string]any{"body": "Do we have capacity this quarter?"}, &edited))
	require.Equal(t, "Do we have capacity this quarter?", edited.Body)

	var resolved model.Comment
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodPatch, "/api/v1/comments/"+thread.ID+"/status",
		map[string]any{"status": "resolved"}, &resolved))
	require.Equal(t, model.StatusResolved, resolved.Status)

	// The list endpoint feeds the idea cards, so the counts include replies.
	var list []model.IdeaSummary
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/ideas", nil, &list))
	require.Equal(t, 2, list[0].CommentCount)

	// Deleting a thread root cascades to its replies.
	require.Equal(t, http.StatusNoContent, call(t, srv, http.MethodDelete, "/api/v1/comments/"+thread.ID, nil, nil))
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID+"/comments", nil, &threads))
	require.Empty(t, threads)
	require.Equal(t, http.StatusNotFound, call(t, srv, http.MethodPut, "/api/v1/comments/"+reply.ID,
		map[string]any{"body": "orphan"}, nil))
}

func TestCommentDetachesWhenAnchoredTextChanges(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Anchored", doc("We should definitely ship this."))

	var thread model.Comment
	require.Equal(t, http.StatusCreated, call(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments",
		map[string]any{
			"body": "Strong word.",
			"anchor": map[string]any{
				"block_index": 0, "start_offset": 10, "end_offset": 20, "snippet": "definitely",
			},
		}, &thread))

	var threads []model.Thread
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID+"/comments", nil, &threads))
	require.False(t, threads[0].Detached)

	// Editing the anchored words away must detach the thread, not delete it.
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodPut, "/api/v1/ideas/"+idea.ID,
		map[string]any{"title": "Anchored", "body": doc("We should ship this.")}, nil))

	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID+"/comments", nil, &threads))
	require.Len(t, threads, 1, "a detached thread stays in the sidebar")
	require.True(t, threads[0].Detached)
	require.NotNil(t, threads[0].Anchor, "the original anchor is kept for reference")

	// Restoring the text re-attaches it, since matching is computed on read.
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodPut, "/api/v1/ideas/"+idea.ID,
		map[string]any{"title": "Anchored", "body": doc("We should definitely ship this.")}, nil))
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID+"/comments", nil, &threads))
	require.False(t, threads[0].Detached)
}

func TestCommentValidation(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Anchored", doc("We should definitely ship this."))
	anchor := map[string]any{"block_index": 0, "start_offset": 10, "end_offset": 20, "snippet": "definitely"}

	// A thread without an anchor is rejected.
	status, code := errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments",
		map[string]any{"body": "floating"})
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "validation_error", code)

	// An anchor whose snippet does not match the body is semantically wrong.
	status, code = errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments",
		map[string]any{"body": "stale", "anchor": map[string]any{
			"block_index": 0, "start_offset": 10, "end_offset": 20, "snippet": "absolutely",
		}})
	require.Equal(t, http.StatusUnprocessableEntity, status)
	require.Equal(t, "unprocessable", code)

	// So is an anchor pointing past the end of the document.
	status, _ = errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments",
		map[string]any{"body": "oob", "anchor": map[string]any{
			"block_index": 9, "start_offset": 0, "end_offset": 4, "snippet": "We s",
		}})
	require.Equal(t, http.StatusUnprocessableEntity, status)

	var thread model.Comment
	require.Equal(t, http.StatusCreated, call(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments",
		map[string]any{"body": "root", "anchor": anchor}, &thread))

	// Replies may not carry their own anchor.
	status, _ = errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments",
		map[string]any{"parent_id": thread.ID, "body": "nested anchor", "anchor": anchor})
	require.Equal(t, http.StatusUnprocessableEntity, status)

	var reply model.Comment
	require.Equal(t, http.StatusCreated, call(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments",
		map[string]any{"parent_id": thread.ID, "body": "fine"}, &reply))

	// Threads are one level deep: a reply cannot be a parent.
	status, _ = errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments",
		map[string]any{"parent_id": reply.ID, "body": "too deep"})
	require.Equal(t, http.StatusUnprocessableEntity, status)

	// A parent on another idea is rejected too.
	other := createIdea(t, srv, "Other", doc("Different text entirely."))
	status, _ = errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+other.ID+"/comments",
		map[string]any{"parent_id": thread.ID, "body": "cross-idea"})
	require.Equal(t, http.StatusUnprocessableEntity, status)

	// An unknown status value is a plain validation error.
	status, _ = errorCode(t, srv, http.MethodPatch, "/api/v1/comments/"+thread.ID+"/status",
		map[string]any{"status": "archived"})
	require.Equal(t, http.StatusBadRequest, status)

	// Empty comment bodies are rejected.
	status, _ = errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments",
		map[string]any{"body": "  ", "anchor": anchor})
	require.Equal(t, http.StatusBadRequest, status)
}

func TestResources(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "With resources", doc("Look at these."))

	var image model.Resource
	require.Equal(t, http.StatusCreated, call(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/resources",
		map[string]any{"type": "image", "url": "https://example.com/shot.png", "label": "Screenshot"}, &image))
	require.Equal(t, model.ResourceImage, image.Type)
	require.NotNil(t, image.Label)
	require.Equal(t, "Screenshot", *image.Label)

	var link model.Resource
	require.Equal(t, http.StatusCreated, call(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/resources",
		map[string]any{"type": "link", "url": "https://example.com/post"}, &link))
	require.Nil(t, link.Label, "an omitted label stays null")

	var resources []model.Resource
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID+"/resources", nil, &resources))
	require.Len(t, resources, 2)

	// The same URL twice on one idea is a conflict.
	status, code := errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/resources",
		map[string]any{"type": "link", "url": "https://example.com/post"})
	require.Equal(t, http.StatusConflict, status)
	require.Equal(t, "conflict", code)

	for _, bad := range []map[string]any{
		{"type": "video", "url": "https://example.com/v.mp4"},
		{"type": "link", "url": "ftp://example.com/file"},
		{"type": "link", "url": "  "},
		{"type": "link", "url": "https://example.com", "label": string(bytes.Repeat([]byte("l"), 201))},
	} {
		status, _ = errorCode(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/resources", bad)
		require.Equal(t, http.StatusBadRequest, status, "payload: %v", bad)
	}

	require.Equal(t, http.StatusNoContent, call(t, srv, http.MethodDelete, "/api/v1/resources/"+link.ID, nil, nil))
	require.Equal(t, http.StatusOK, call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID+"/resources", nil, &resources))
	require.Len(t, resources, 1)
	require.Equal(t, http.StatusNotFound, call(t, srv, http.MethodDelete, "/api/v1/resources/"+link.ID, nil, nil))
}

func TestDeletingAnIdeaCascades(t *testing.T) {
	srv := newTestServer(t)
	idea := createIdea(t, srv, "Doomed", doc("We should definitely ship this."))

	var thread model.Comment
	require.Equal(t, http.StatusCreated, call(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments",
		map[string]any{"body": "note", "anchor": map[string]any{
			"block_index": 0, "start_offset": 10, "end_offset": 20, "snippet": "definitely",
		}}, &thread))
	require.Equal(t, http.StatusCreated, call(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/comments",
		map[string]any{"parent_id": thread.ID, "body": "reply"}, nil))

	var resource model.Resource
	require.Equal(t, http.StatusCreated, call(t, srv, http.MethodPost, "/api/v1/ideas/"+idea.ID+"/resources",
		map[string]any{"type": "link", "url": "https://example.com"}, &resource))

	require.Equal(t, http.StatusNoContent, call(t, srv, http.MethodDelete, "/api/v1/ideas/"+idea.ID, nil, nil))

	// Children are gone with the parent, not merely hidden.
	var remaining int
	require.NoError(t, testDB.Pool.QueryRow(t.Context(),
		`select (select count(*) from comments) + (select count(*) from resources)`).Scan(&remaining))
	require.Zero(t, remaining)

	require.Equal(t, http.StatusNotFound, call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID+"/comments", nil, nil))
	require.Equal(t, http.StatusNotFound, call(t, srv, http.MethodGet, "/api/v1/ideas/"+idea.ID+"/resources", nil, nil))
}

func TestEnvelopesAndHealth(t *testing.T) {
	srv := newTestServer(t)

	res, err := srv.Client().Get(srv.URL + "/healthz")
	require.NoError(t, err)
	defer func() { require.NoError(t, res.Body.Close()) }()
	require.Equal(t, http.StatusOK, res.StatusCode)

	raw, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"data":{"status":"ok"}}`, string(raw))

	status, code := errorCode(t, srv, http.MethodGet, "/api/v1/nope", nil)
	require.Equal(t, http.StatusNotFound, status)
	require.Equal(t, "not_found", code)

	// Unknown fields are a client bug worth surfacing, not something to ignore.
	status, _ = errorCode(t, srv, http.MethodPost, "/api/v1/ideas",
		map[string]any{"title": "ok", "titel": "typo"})
	require.Equal(t, http.StatusBadRequest, status)
}
