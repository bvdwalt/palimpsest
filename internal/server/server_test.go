package server_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvdwalt/inkbase/internal/db"
	"github.com/bvdwalt/inkbase/internal/server"
	"github.com/bvdwalt/inkbase/internal/store"
	"github.com/bvdwalt/inkbase/web"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	sqlDB, err := db.Connect(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return server.New(store.New(sqlDB), 10)
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func createPage(t *testing.T, h http.Handler, parentID *string, title string) store.Page {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/api/pages", map[string]any{
		"parentId":    parentID,
		"title":       title,
		"contentJson": "",
		"contentText": "",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %q: status = %d, body = %s", title, rec.Code, rec.Body.String())
	}
	var page store.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return page
}

func TestHealth(t *testing.T) {
	h := newTestServer(t)
	rec := doJSON(t, h, http.MethodGet, "/health", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestCreateGetListDeletePage(t *testing.T) {
	h := newTestServer(t)

	page := createPage(t, h, nil, "Homelab")

	rec := doJSON(t, h, http.MethodGet, "/api/pages/"+page.ID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/api/pages", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status = %d", rec.Code)
	}
	var pages []store.PageSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &pages); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("len(pages) = %d, want 1", len(pages))
	}

	rec = doJSON(t, h, http.MethodDelete, "/api/pages/"+page.ID, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/api/pages/"+page.ID, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("get after delete: status = %d, want 404", rec.Code)
	}
}

func TestGetPageNotFound(t *testing.T) {
	h := newTestServer(t)
	rec := doJSON(t, h, http.MethodGet, "/api/pages/missing", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestCreatePageRequiresTitle(t *testing.T) {
	h := newTestServer(t)
	rec := doJSON(t, h, http.MethodPost, "/api/pages", map[string]any{"title": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestUpdatePage(t *testing.T) {
	h := newTestServer(t)
	page := createPage(t, h, nil, "Doc")

	rec := doJSON(t, h, http.MethodPut, "/api/pages/"+page.ID, map[string]any{
		"title":       "Doc v2",
		"parentId":    nil,
		"contentJson": "{}",
		"contentText": "updated",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var updated store.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated.Title != "Doc v2" || updated.ContentText != "updated" {
		t.Errorf("updated = %+v", updated)
	}
}

func TestUpdatePageRequiresTitle(t *testing.T) {
	h := newTestServer(t)
	page := createPage(t, h, nil, "Doc")

	rec := doJSON(t, h, http.MethodPut, "/api/pages/"+page.ID, map[string]any{
		"title":       "",
		"parentId":    nil,
		"contentJson": "",
		"contentText": "",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestUpdatePageRejectsCycle(t *testing.T) {
	h := newTestServer(t)
	parent := createPage(t, h, nil, "Parent")
	child := createPage(t, h, &parent.ID, "Child")

	rec := doJSON(t, h, http.MethodPut, "/api/pages/"+parent.ID, map[string]any{
		"title":       "Parent",
		"parentId":    child.ID,
		"contentJson": "",
		"contentText": "",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestUpdatePageNotFound(t *testing.T) {
	h := newTestServer(t)
	rec := doJSON(t, h, http.MethodPut, "/api/pages/missing", map[string]any{
		"title":       "Title",
		"parentId":    nil,
		"contentJson": "",
		"contentText": "",
	})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
}

func TestMovePage(t *testing.T) {
	h := newTestServer(t)
	parent := createPage(t, h, nil, "Parent")
	child := createPage(t, h, nil, "Child")

	rec := doJSON(t, h, http.MethodPatch, "/api/pages/"+child.ID+"/parent", map[string]any{
		"parentId": parent.ID,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var moved store.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &moved); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if moved.ParentID == nil || *moved.ParentID != parent.ID {
		t.Errorf("ParentID = %v, want %q", moved.ParentID, parent.ID)
	}

	rec = doJSON(t, h, http.MethodPatch, "/api/pages/"+child.ID+"/parent", map[string]any{
		"parentId": nil,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("move to top level: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &moved); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if moved.ParentID != nil {
		t.Errorf("ParentID = %v, want nil", moved.ParentID)
	}
}

func TestMovePageRejectsCycle(t *testing.T) {
	h := newTestServer(t)
	parent := createPage(t, h, nil, "Parent")
	child := createPage(t, h, &parent.ID, "Child")

	rec := doJSON(t, h, http.MethodPatch, "/api/pages/"+parent.ID+"/parent", map[string]any{
		"parentId": child.ID,
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestMovePageNotFound(t *testing.T) {
	h := newTestServer(t)
	rec := doJSON(t, h, http.MethodPatch, "/api/pages/missing/parent", map[string]any{
		"parentId": nil,
	})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
}

func TestGetConfig(t *testing.T) {
	h := newTestServer(t)
	rec := doJSON(t, h, http.MethodGet, "/api/config", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var cfg map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg["autosaveIntervalSeconds"] != 10 {
		t.Errorf("autosaveIntervalSeconds = %d, want 10", cfg["autosaveIntervalSeconds"])
	}
}

func TestListRevisionsAndRevert(t *testing.T) {
	h := newTestServer(t)
	page := createPage(t, h, nil, "Doc")

	// No revisions yet.
	rec := doJSON(t, h, http.MethodGet, "/api/pages/"+page.ID+"/revisions", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var revs []store.Revision
	if err := json.Unmarshal(rec.Body.Bytes(), &revs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(revs) != 0 {
		t.Fatalf("len(revs) = %d, want 0", len(revs))
	}

	// Editing the page snapshots the old content into a revision.
	rec = doJSON(t, h, http.MethodPut, "/api/pages/"+page.ID, map[string]any{
		"title":       "Doc",
		"parentId":    nil,
		"contentJson": "{}",
		"contentText": "v2",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/api/pages/"+page.ID+"/revisions", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &revs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("len(revs) = %d, want 1", len(revs))
	}

	rec = doJSON(t, h, http.MethodPost, "/api/pages/"+page.ID+"/revert/"+revs[0].ID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("revert: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var reverted store.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &reverted); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if reverted.ContentText != "" {
		t.Errorf("ContentText = %q, want %q (reverted to pre-edit content)", reverted.ContentText, "")
	}
}

func TestRevertNotFound(t *testing.T) {
	h := newTestServer(t)
	page := createPage(t, h, nil, "Doc")

	rec := doJSON(t, h, http.MethodPost, "/api/pages/"+page.ID+"/revert/missing-revision", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
}

func TestSearchEndpoint(t *testing.T) {
	h := newTestServer(t)
	createPage(t, h, nil, "Kubernetes Notes")

	rec := doJSON(t, h, http.MethodGet, "/api/search?q=kubernetes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var results []store.SearchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}
}

func TestSearchEmptyQueryReturnsEmptyResults(t *testing.T) {
	h := newTestServer(t)
	rec := doJSON(t, h, http.MethodGet, "/api/search", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var results []store.SearchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestCreatePageInvalidJSON(t *testing.T) {
	h := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/pages", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestUpdatePageInvalidJSON(t *testing.T) {
	h := newTestServer(t)
	page := createPage(t, h, nil, "Doc")

	req := httptest.NewRequest(http.MethodPut, "/api/pages/"+page.ID, bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestMovePageInvalidJSON(t *testing.T) {
	h := newTestServer(t)
	page := createPage(t, h, nil, "Doc")

	req := httptest.NewRequest(http.MethodPatch, "/api/pages/"+page.ID+"/parent", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestDeletePageNotFound(t *testing.T) {
	h := newTestServer(t)
	rec := doJSON(t, h, http.MethodDelete, "/api/pages/missing", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
}

func TestSPAFallbackServesIndexForUnknownRoute(t *testing.T) {
	h := newTestServer(t)
	rec := doJSON(t, h, http.MethodGet, "/some/client/route", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<div id=\"root\">") {
		t.Errorf("body doesn't look like index.html: %s", rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
	}
}

func TestSPARootPathServesIndex(t *testing.T) {
	h := newTestServer(t)
	rec := doJSON(t, h, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<div id=\"root\">") {
		t.Errorf("body doesn't look like index.html: %s", rec.Body.String())
	}
}

func TestSPAServesTopLevelFileWithNoCache(t *testing.T) {
	h := newTestServer(t)
	rec := doJSON(t, h, http.MethodGet, "/favicon.svg", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
	}
}

// Closing the DB out from under a live server forces real (unmocked) driver
// errors, exercising the handlers' generic-500 fallback paths.
func TestHandlersReturn500OnStoreFailure(t *testing.T) {
	sqlDB, err := db.Connect(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	h := server.New(store.New(sqlDB), 10)
	page := createPage(t, h, nil, "Doc")
	sqlDB.Close()

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"list", http.MethodGet, "/api/pages", nil},
		{"create", http.MethodPost, "/api/pages", map[string]any{"title": "New"}},
		{"update", http.MethodPut, "/api/pages/" + page.ID, map[string]any{"title": "Doc", "parentId": nil}},
		{"delete", http.MethodDelete, "/api/pages/" + page.ID, nil},
		{"revisions", http.MethodGet, "/api/pages/" + page.ID + "/revisions", nil},
		{"search", http.MethodGet, "/api/search?q=doc", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := doJSON(t, h, c.method, c.path, c.body)
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want 500, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSPAServesHashedAssetWithLongCache(t *testing.T) {
	h := newTestServer(t)

	// Asset filenames are content-hashed by the Vite build, so discover one instead of hardcoding it.
	entries, err := fs.ReadDir(web.FS, "dist/assets")
	if err != nil || len(entries) == 0 {
		t.Skip("no built assets in web/dist/assets to test against")
	}
	assetName := entries[0].Name()

	rec := doJSON(t, h, http.MethodGet, "/assets/"+assetName, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want long-cache directive", got)
	}
}
