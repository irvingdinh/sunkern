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

// timeFormat is the canonical SQLite TEXT timestamp format used by the db
// package for writing timestamps (Model, FormatTime). Existing data and
// SQLite's datetime('now') DEFAULT produce this format.
const timeFormat = "2006-01-02 15:04:05"

// timeFormatMs is the timestamp format with millisecond precision. The CGo
// driver uses this for time.Time bindings (framework/sqlite/driver). The
// scan layer tries both formats when parsing timestamps.
const timeFormatMs = "2006-01-02 15:04:05.000"

// ErrNotFound is returned by QueryOne/ScanOne when no rows match the query.
var ErrNotFound = errors.New("db: not found")

// Querier is satisfied by both *sql.DB and *sql.Tx, so query builders work
// transparently inside and outside transactions.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
