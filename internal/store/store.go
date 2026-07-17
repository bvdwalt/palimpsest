package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")
var ErrCycle = errors.New("cannot move a page under itself or one of its own descendants")

type Store struct {
	db dbConn
}

func New(db *sql.DB) *Store {
	return &Store{db: sqlDB{db}}
}

type PageSummary struct {
	ID       string  `json:"id"`
	ParentID *string `json:"parentId"`
	Slug     string  `json:"slug"`
	Title    string  `json:"title"`
}

type Page struct {
	ID          string    `json:"id"`
	ParentID    *string   `json:"parentId"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	ContentJSON string    `json:"contentJson"`
	ContentText string    `json:"contentText"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Revision struct {
	ID          string    `json:"id"`
	PageID      string    `json:"pageId"`
	Title       string    `json:"title"`
	ContentJSON string    `json:"contentJson"`
	ContentText string    `json:"contentText"`
	CreatedAt   time.Time `json:"createdAt"`
}

type SearchResult struct {
	ID      string `json:"id"`
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

func (s *Store) ListTree(ctx context.Context) ([]PageSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, parent_id, slug, title FROM pages ORDER BY title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pages := []PageSummary{}
	for rows.Next() {
		var p PageSummary
		if err := rows.Scan(&p.ID, &p.ParentID, &p.Slug, &p.Title); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

func (s *Store) Get(ctx context.Context, id string) (*Page, error) {
	var p Page
	err := s.db.QueryRowContext(ctx, `
		SELECT id, parent_id, slug, title, content_json, content_text, created_at, updated_at
		FROM pages WHERE id = ?
	`, id).Scan(&p.ID, &p.ParentID, &p.Slug, &p.Title, &p.ContentJSON, &p.ContentText, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) Create(ctx context.Context, parentID *string, title, contentJSON, contentText string) (*Page, error) {
	slug, err := s.uniqueSlug(ctx, title, "")
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO pages (id, parent_id, slug, title, content_json, content_text)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, parentID, slug, title, contentJSON, contentText); err != nil {
		return nil, err
	}

	return s.Get(ctx, id)
}

// Update snapshots current content into revisions, regenerates the slug, and applies the new content; parentID nil moves the page to the top level.
func (s *Store) Update(ctx context.Context, id string, title string, parentID *string, contentJSON, contentText string) (*Page, error) {
	if parentID != nil {
		cyclic, err := s.wouldCreateCycle(ctx, id, *parentID)
		if err != nil {
			return nil, err
		}
		if cyclic {
			return nil, ErrCycle
		}
	}

	slug, err := s.uniqueSlug(ctx, title, id)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var curTitle, curContentJSON, curContentText string
	err = tx.QueryRowContext(ctx, `SELECT title, content_json, content_text FROM pages WHERE id = ?`, id).
		Scan(&curTitle, &curContentJSON, &curContentText)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO revisions (id, page_id, title, content_json, content_text)
		VALUES (?, ?, ?, ?, ?)
	`, uuid.NewString(), id, curTitle, curContentJSON, curContentText); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE pages SET title = ?, slug = ?, parent_id = ?, content_json = ?, content_text = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, title, slug, parentID, contentJSON, contentText, id); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// Move reparents a page without touching content or revisions; parentID nil moves the page to the top level.
func (s *Store) Move(ctx context.Context, id string, parentID *string) (*Page, error) {
	if parentID != nil {
		cyclic, err := s.wouldCreateCycle(ctx, id, *parentID)
		if err != nil {
			return nil, err
		}
		if cyclic {
			return nil, ErrCycle
		}
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE pages SET parent_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, parentID, id)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotFound
	}

	return s.Get(ctx, id)
}

// wouldCreateCycle reports whether newParentID is id itself or a descendant of id.
func (s *Store) wouldCreateCycle(ctx context.Context, id, newParentID string) (bool, error) {
	if id == newParentID {
		return true, nil
	}

	current := newParentID
	for {
		var parent sql.NullString
		err := s.db.QueryRowContext(ctx, `SELECT parent_id FROM pages WHERE id = ?`, current).Scan(&parent)
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrNotFound
		}
		if err != nil {
			return false, err
		}
		if !parent.Valid {
			return false, nil
		}
		if parent.String == id {
			return true, nil
		}
		current = parent.String
	}
}

func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM pages WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListRevisions(ctx context.Context, pageID string) ([]Revision, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, page_id, title, content_json, content_text, created_at
		FROM revisions WHERE page_id = ? ORDER BY created_at DESC
	`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	revs := []Revision{}
	for rows.Next() {
		var r Revision
		if err := rows.Scan(&r.ID, &r.PageID, &r.Title, &r.ContentJSON, &r.ContentText, &r.CreatedAt); err != nil {
			return nil, err
		}
		revs = append(revs, r)
	}
	return revs, rows.Err()
}

// Revert snapshots current content into revisions, then restores content from the given past revision.
func (s *Store) Revert(ctx context.Context, pageID, revisionID string) (*Page, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var revTitle, revContentJSON, revContentText string
	err = tx.QueryRowContext(ctx, `
		SELECT title, content_json, content_text FROM revisions WHERE id = ? AND page_id = ?
	`, revisionID, pageID).Scan(&revTitle, &revContentJSON, &revContentText)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var curTitle, curContentJSON, curContentText string
	err = tx.QueryRowContext(ctx, `SELECT title, content_json, content_text FROM pages WHERE id = ?`, pageID).
		Scan(&curTitle, &curContentJSON, &curContentText)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO revisions (id, page_id, title, content_json, content_text)
		VALUES (?, ?, ?, ?, ?)
	`, uuid.NewString(), pageID, curTitle, curContentJSON, curContentText); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE pages SET title = ?, content_json = ?, content_text = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, revTitle, revContentJSON, revContentText, pageID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.Get(ctx, pageID)
}

func (s *Store) Search(ctx context.Context, query string) ([]SearchResult, error) {
	// Quoted as a literal phrase so it's not parsed as FTS5 query syntax.
	ftsQuery := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`

	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.slug, p.title, snippet(pages_fts, 2, '<mark>', '</mark>', '...', 12)
		FROM pages_fts
		JOIN pages p ON p.id = pages_fts.page_id
		WHERE pages_fts MATCH ?
		ORDER BY rank
		LIMIT 50
	`, ftsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []SearchResult{}
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Slug, &r.Title, &r.Snippet); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

var slugInvalidChars = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	s := slugInvalidChars.ReplaceAllString(strings.ToLower(title), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "page"
	}
	return s
}

// uniqueSlug appends -2, -3, ... until unique; excludeID (pass "" when creating) excludes the page's own row from the collision check.
func (s *Store) uniqueSlug(ctx context.Context, title string, excludeID string) (string, error) {
	base := slugify(title)
	slug := base
	for n := 2; ; n++ {
		var exists bool
		err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pages WHERE slug = ? AND id != ?)`, slug, excludeID).Scan(&exists)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, n)
	}
}
