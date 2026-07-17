package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/bvdwalt/inkbase/internal/db"
	"github.com/bvdwalt/inkbase/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	sqlDB, err := db.Connect(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return store.New(sqlDB)
}

func strPtr(s string) *string { return &s }

func TestCreateAndGet(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	page, err := s.Create(ctx, nil, "Homelab", "{}", "hello")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if page.Slug != "homelab" {
		t.Errorf("Slug = %q, want %q", page.Slug, "homelab")
	}
	if page.ParentID != nil {
		t.Errorf("ParentID = %v, want nil", page.ParentID)
	}

	got, err := s.Get(ctx, page.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Homelab" {
		t.Errorf("Title = %q, want %q", got.Title, "Homelab")
	}
}

func TestCreateDuplicateTitleGetsUniqueSlug(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	a, err := s.Create(ctx, nil, "Notes", "", "")
	if err != nil {
		t.Fatalf("Create a: %v", err)
	}
	b, err := s.Create(ctx, nil, "Notes", "", "")
	if err != nil {
		t.Fatalf("Create b: %v", err)
	}
	if a.Slug == b.Slug {
		t.Errorf("expected distinct slugs, both were %q", a.Slug)
	}
}

func TestGetNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, err := s.Get(ctx, "missing"); err != store.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateReparentsAndSnapshotsRevision(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	parent, err := s.Create(ctx, nil, "Parent", "", "")
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	child, err := s.Create(ctx, nil, "Child", "old json", "old text")
	if err != nil {
		t.Fatalf("Create child: %v", err)
	}

	updated, err := s.Update(ctx, child.ID, "Child", strPtr(parent.ID), "new json", "new text")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.ParentID == nil || *updated.ParentID != parent.ID {
		t.Errorf("ParentID = %v, want %q", updated.ParentID, parent.ID)
	}
	if updated.ContentText != "new text" {
		t.Errorf("ContentText = %q, want %q", updated.ContentText, "new text")
	}

	revs, err := s.ListRevisions(ctx, child.ID)
	if err != nil {
		t.Fatalf("ListRevisions: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("len(revs) = %d, want 1", len(revs))
	}
	if revs[0].ContentText != "old text" {
		t.Errorf("revision snapshot ContentText = %q, want %q", revs[0].ContentText, "old text")
	}
}

func TestUpdateRejectsCycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	parent, err := s.Create(ctx, nil, "Parent", "", "")
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	child, err := s.Create(ctx, strPtr(parent.ID), "Child", "", "")
	if err != nil {
		t.Fatalf("Create child: %v", err)
	}

	if _, err := s.Update(ctx, parent.ID, "Parent", strPtr(child.ID), "", ""); err != store.ErrCycle {
		t.Errorf("err = %v, want ErrCycle", err)
	}
}

func TestMoveReparentsWithoutRevisionOrContentChange(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	parent, err := s.Create(ctx, nil, "Parent", "", "")
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	child, err := s.Create(ctx, nil, "Child", "content json", "content text")
	if err != nil {
		t.Fatalf("Create child: %v", err)
	}

	moved, err := s.Move(ctx, child.ID, strPtr(parent.ID))
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if moved.ParentID == nil || *moved.ParentID != parent.ID {
		t.Errorf("ParentID = %v, want %q", moved.ParentID, parent.ID)
	}
	if moved.ContentText != "content text" {
		t.Errorf("ContentText changed by Move: got %q", moved.ContentText)
	}

	revs, err := s.ListRevisions(ctx, child.ID)
	if err != nil {
		t.Fatalf("ListRevisions: %v", err)
	}
	if len(revs) != 0 {
		t.Errorf("len(revs) = %d, want 0 (Move should not snapshot revisions)", len(revs))
	}

	backToTop, err := s.Move(ctx, child.ID, nil)
	if err != nil {
		t.Fatalf("Move to top level: %v", err)
	}
	if backToTop.ParentID != nil {
		t.Errorf("ParentID = %v, want nil after moving to top level", backToTop.ParentID)
	}
}

func TestMoveRejectsCycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	grandparent, err := s.Create(ctx, nil, "Grandparent", "", "")
	if err != nil {
		t.Fatalf("Create grandparent: %v", err)
	}
	parent, err := s.Create(ctx, strPtr(grandparent.ID), "Parent", "", "")
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	child, err := s.Create(ctx, strPtr(parent.ID), "Child", "", "")
	if err != nil {
		t.Fatalf("Create child: %v", err)
	}

	// A page can't become its own parent...
	if _, err := s.Move(ctx, parent.ID, strPtr(parent.ID)); err != store.ErrCycle {
		t.Errorf("self-parent: err = %v, want ErrCycle", err)
	}
	// ...nor move under its own descendant.
	if _, err := s.Move(ctx, grandparent.ID, strPtr(child.ID)); err != store.ErrCycle {
		t.Errorf("descendant-parent: err = %v, want ErrCycle", err)
	}
}

func TestMoveNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, err := s.Move(ctx, "missing", nil); err != store.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	page, err := s.Create(ctx, nil, "Temp", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Delete(ctx, page.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, page.ID); err != store.ErrNotFound {
		t.Errorf("Get after delete: err = %v, want ErrNotFound", err)
	}
	if err := s.Delete(ctx, page.ID); err != store.ErrNotFound {
		t.Errorf("Delete missing: err = %v, want ErrNotFound", err)
	}
}

func TestListTree(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	parent, err := s.Create(ctx, nil, "Parent", "", "")
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	if _, err := s.Create(ctx, strPtr(parent.ID), "Child", "", ""); err != nil {
		t.Fatalf("Create child: %v", err)
	}

	pages, err := s.ListTree(ctx)
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("len(pages) = %d, want 2", len(pages))
	}
}

func TestSearch(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, err := s.Create(ctx, nil, "Kubernetes Notes", "", "notes about pods and deployments"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.Create(ctx, nil, "Grocery List", "", "milk and eggs"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	results, err := s.Search(ctx, "pods")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Title != "Kubernetes Notes" {
		t.Errorf("Title = %q, want %q", results[0].Title, "Kubernetes Notes")
	}
}

func TestRevertRestoresContentAndSnapshotsCurrent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	page, err := s.Create(ctx, nil, "Doc", "v1 json", "v1 text")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	updated, err := s.Update(ctx, page.ID, "Doc", nil, "v2 json", "v2 text")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	revs, err := s.ListRevisions(ctx, page.ID)
	if err != nil {
		t.Fatalf("ListRevisions: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("len(revs) = %d, want 1", len(revs))
	}
	v1RevisionID := revs[0].ID

	reverted, err := s.Revert(ctx, updated.ID, v1RevisionID)
	if err != nil {
		t.Fatalf("Revert: %v", err)
	}
	if reverted.ContentText != "v1 text" {
		t.Errorf("ContentText = %q, want %q", reverted.ContentText, "v1 text")
	}

	revsAfter, err := s.ListRevisions(ctx, page.ID)
	if err != nil {
		t.Fatalf("ListRevisions after revert: %v", err)
	}
	if len(revsAfter) != 2 {
		t.Errorf("len(revsAfter) = %d, want 2 (revert snapshots the pre-revert state)", len(revsAfter))
	}
}
