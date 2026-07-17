package store

import (
	"context"
	"database/sql"
)

// Mirrors the subset of database/sql Store actually uses, so tests can hand
// Store a fake and trigger error paths a real SQLite file can't (a failed
// COMMIT, a broken connection mid-query). sqlDB/sqlTx adapt the real types.
type row interface {
	Scan(dest ...any) error
}

type rowsIface interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
	Err() error
}

type dbTx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) row
	Commit() error
	Rollback() error
}

type dbConn interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (rowsIface, error)
	QueryRowContext(ctx context.Context, query string, args ...any) row
	BeginTx(ctx context.Context, opts *sql.TxOptions) (dbTx, error)
}

// sqlDB adapts *sql.DB to dbConn: the real methods already match structurally
// except for the return types this file narrows to interfaces.
type sqlDB struct{ *sql.DB }

func (d sqlDB) QueryContext(ctx context.Context, query string, args ...any) (rowsIface, error) {
	return d.DB.QueryContext(ctx, query, args...)
}

func (d sqlDB) QueryRowContext(ctx context.Context, query string, args ...any) row {
	return d.DB.QueryRowContext(ctx, query, args...)
}

func (d sqlDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (dbTx, error) {
	tx, err := d.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return sqlTx{tx}, nil
}

// sqlTx adapts *sql.Tx to dbTx the same way sqlDB adapts *sql.DB.
type sqlTx struct{ *sql.Tx }

func (t sqlTx) QueryRowContext(ctx context.Context, query string, args ...any) row {
	return t.Tx.QueryRowContext(ctx, query, args...)
}
