package store

import (
	"context"
	"database/sql"
)

// fakeResult is a scriptable sql.Result.
type fakeResult struct {
	rows    int64
	rowsErr error
}

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }

func (r fakeResult) RowsAffected() (int64, error) {
	if r.rowsErr != nil {
		return 0, r.rowsErr
	}
	return r.rows, nil
}

// fakeRow is a scriptable row.
type fakeRow struct{ err error }

func (r fakeRow) Scan(dest ...any) error { return r.err }

// fakeRows yields n rows, failing Scan at row scanAt (if scanErr is set) or
// Err() once exhausted (if errAfter is set).
type fakeRows struct {
	n        int
	scanErr  error
	scanAt   int
	errAfter error

	i int
}

func (r *fakeRows) Next() bool {
	if r.i >= r.n {
		return false
	}
	r.i++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.scanErr != nil && r.i == r.scanAt {
		return r.scanErr
	}
	return nil
}

func (r *fakeRows) Close() error { return nil }

func (r *fakeRows) Err() error { return r.errAfter }

// fakeTx scripts a dbTx. execErrs/queryRowErrs are consumed in call order
// (nil, or running past the slice, succeeds), so a test can target one call
// in a sequence (e.g. Update's revision-insert vs. its page-update) independently.
type fakeTx struct {
	execErrs     []error
	execCall     int
	queryRowErrs []error
	queryRowCall int
	commitErr    error
	rollbackErr  error
}

func (f *fakeTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	var err error
	if f.execCall < len(f.execErrs) {
		err = f.execErrs[f.execCall]
	}
	f.execCall++
	if err != nil {
		return nil, err
	}
	return fakeResult{rows: 1}, nil
}

func (f *fakeTx) QueryRowContext(ctx context.Context, query string, args ...any) row {
	var err error
	if f.queryRowCall < len(f.queryRowErrs) {
		err = f.queryRowErrs[f.queryRowCall]
	}
	f.queryRowCall++
	return fakeRow{err: err}
}

func (f *fakeTx) Commit() error { return f.commitErr }

func (f *fakeTx) Rollback() error { return f.rollbackErr }

// fakeConn scripts a dbConn.
type fakeConn struct {
	execResult fakeResult
	execErr    error

	queryRows *fakeRows
	queryErr  error

	queryRowErr error

	beginErr error
	tx       *fakeTx
}

func (f *fakeConn) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if f.execErr != nil {
		return nil, f.execErr
	}
	return f.execResult, nil
}

func (f *fakeConn) QueryContext(ctx context.Context, query string, args ...any) (rowsIface, error) {
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.queryRows != nil {
		return f.queryRows, nil
	}
	return &fakeRows{}, nil
}

func (f *fakeConn) QueryRowContext(ctx context.Context, query string, args ...any) row {
	return fakeRow{err: f.queryRowErr}
}

func (f *fakeConn) BeginTx(ctx context.Context, opts *sql.TxOptions) (dbTx, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}
