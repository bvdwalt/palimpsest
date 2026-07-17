package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

var errDB = errors.New("boom")

func TestCreateUniqueSlugQueryError(t *testing.T) {
	s := &Store{db: &fakeConn{queryRowErr: errDB}}
	if _, err := s.Create(context.Background(), nil, "Doc", "", ""); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestCreateInsertExecError(t *testing.T) {
	s := &Store{db: &fakeConn{execErr: errDB}}
	if _, err := s.Create(context.Background(), nil, "Doc", "", ""); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestGetQueryError(t *testing.T) {
	s := &Store{db: &fakeConn{queryRowErr: errDB}}
	if _, err := s.Get(context.Background(), "id"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestUpdateUniqueSlugQueryError(t *testing.T) {
	s := &Store{db: &fakeConn{queryRowErr: errDB}}
	if _, err := s.Update(context.Background(), "id", "Doc", nil, "", ""); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestUpdateBeginTxError(t *testing.T) {
	s := &Store{db: &fakeConn{beginErr: errDB}}
	if _, err := s.Update(context.Background(), "id", "Doc", nil, "", ""); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestUpdateSelectCurrentError(t *testing.T) {
	s := &Store{db: &fakeConn{tx: &fakeTx{queryRowErrs: []error{errDB}}}}
	if _, err := s.Update(context.Background(), "id", "Doc", nil, "", ""); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestUpdateInsertRevisionExecError(t *testing.T) {
	s := &Store{db: &fakeConn{tx: &fakeTx{execErrs: []error{errDB}}}}
	if _, err := s.Update(context.Background(), "id", "Doc", nil, "", ""); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestUpdateApplyExecError(t *testing.T) {
	s := &Store{db: &fakeConn{tx: &fakeTx{execErrs: []error{nil, errDB}}}}
	if _, err := s.Update(context.Background(), "id", "Doc", nil, "", ""); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestUpdateCommitError(t *testing.T) {
	s := &Store{db: &fakeConn{tx: &fakeTx{commitErr: errDB}}}
	if _, err := s.Update(context.Background(), "id", "Doc", nil, "", ""); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestMoveExecError(t *testing.T) {
	s := &Store{db: &fakeConn{execErr: errDB}}
	if _, err := s.Move(context.Background(), "id", nil); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestMoveRowsAffectedError(t *testing.T) {
	s := &Store{db: &fakeConn{execResult: fakeResult{rowsErr: errDB}}}
	if _, err := s.Move(context.Background(), "id", nil); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestWouldCreateCycleGenericQueryError(t *testing.T) {
	s := &Store{db: &fakeConn{queryRowErr: errDB}}
	if _, err := s.wouldCreateCycle(context.Background(), "a", "b"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestDeleteExecError(t *testing.T) {
	s := &Store{db: &fakeConn{execErr: errDB}}
	if err := s.Delete(context.Background(), "id"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestDeleteRowsAffectedError(t *testing.T) {
	s := &Store{db: &fakeConn{execResult: fakeResult{rowsErr: errDB}}}
	if err := s.Delete(context.Background(), "id"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestListTreeQueryError(t *testing.T) {
	s := &Store{db: &fakeConn{queryErr: errDB}}
	if _, err := s.ListTree(context.Background()); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestListTreeScanError(t *testing.T) {
	s := &Store{db: &fakeConn{queryRows: &fakeRows{n: 1, scanAt: 1, scanErr: errDB}}}
	if _, err := s.ListTree(context.Background()); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestListTreeRowsErr(t *testing.T) {
	s := &Store{db: &fakeConn{queryRows: &fakeRows{errAfter: errDB}}}
	if _, err := s.ListTree(context.Background()); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestListRevisionsQueryError(t *testing.T) {
	s := &Store{db: &fakeConn{queryErr: errDB}}
	if _, err := s.ListRevisions(context.Background(), "id"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestListRevisionsScanError(t *testing.T) {
	s := &Store{db: &fakeConn{queryRows: &fakeRows{n: 1, scanAt: 1, scanErr: errDB}}}
	if _, err := s.ListRevisions(context.Background(), "id"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestRevertBeginTxError(t *testing.T) {
	s := &Store{db: &fakeConn{beginErr: errDB}}
	if _, err := s.Revert(context.Background(), "page", "rev"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestRevertLookupRevisionError(t *testing.T) {
	s := &Store{db: &fakeConn{tx: &fakeTx{queryRowErrs: []error{errDB}}}}
	if _, err := s.Revert(context.Background(), "page", "rev"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestRevertLookupCurrentError(t *testing.T) {
	s := &Store{db: &fakeConn{tx: &fakeTx{queryRowErrs: []error{nil, errDB}}}}
	if _, err := s.Revert(context.Background(), "page", "rev"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestRevertLookupCurrentNotFound(t *testing.T) {
	s := &Store{db: &fakeConn{tx: &fakeTx{queryRowErrs: []error{nil, sql.ErrNoRows}}}}
	if _, err := s.Revert(context.Background(), "page", "rev"); err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRevertInsertRevisionExecError(t *testing.T) {
	s := &Store{db: &fakeConn{tx: &fakeTx{execErrs: []error{errDB}}}}
	if _, err := s.Revert(context.Background(), "page", "rev"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestRevertApplyExecError(t *testing.T) {
	s := &Store{db: &fakeConn{tx: &fakeTx{execErrs: []error{nil, errDB}}}}
	if _, err := s.Revert(context.Background(), "page", "rev"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestRevertCommitError(t *testing.T) {
	s := &Store{db: &fakeConn{tx: &fakeTx{commitErr: errDB}}}
	if _, err := s.Revert(context.Background(), "page", "rev"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestSearchQueryError(t *testing.T) {
	s := &Store{db: &fakeConn{queryErr: errDB}}
	if _, err := s.Search(context.Background(), "q"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestSearchScanError(t *testing.T) {
	s := &Store{db: &fakeConn{queryRows: &fakeRows{n: 1, scanAt: 1, scanErr: errDB}}}
	if _, err := s.Search(context.Background(), "q"); !errors.Is(err, errDB) {
		t.Errorf("err = %v, want errDB", err)
	}
}

func TestSlugifyEmptyResultFallsBackToPage(t *testing.T) {
	if got := slugify("!!!"); got != "page" {
		t.Errorf("slugify(%q) = %q, want %q", "!!!", got, "page")
	}
}
