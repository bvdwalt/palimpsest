package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/bvdwalt/inkbase/internal/db"
	"github.com/bvdwalt/inkbase/internal/server"
	"github.com/bvdwalt/inkbase/internal/store"
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
