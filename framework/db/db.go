// Package db provides a schema-as-code query builder for SQLite. It builds
// on top of framework/sqlite and accepts the Querier interface so that all
// query builders work transparently with both *sql.DB and *sql.Tx.
//
// The package is a pure library — no global state, no Load/Global pattern.
// Modules resolve *sqlite.DB from the container and pass ReadDB()/WriteDB()
// to the query builders directly.
package db

import (
	"context"
	"database/sql"
	"errors"
)

// timeFormat is the canonical SQLite TEXT timestamp format. It matches the
// CGo driver's time.Time bind format (framework/sqlite/driver/driver.go:255).
const timeFormat = "2006-01-02 15:04:05"

// ErrNotFound is returned by QueryOne/ScanOne when no rows match the query.
var ErrNotFound = errors.New("db: not found")

// Querier is satisfied by both *sql.DB and *sql.Tx, so query builders work
// transparently inside and outside transactions.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
