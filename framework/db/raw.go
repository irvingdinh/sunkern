package db

import (
	"context"
	"database/sql"
	"fmt"
)

// RawExec executes arbitrary SQL with args. Use for DDL, PRAGMA, or
// statements the builder cannot express.
func RawExec(ctx context.Context, q Querier, query string, args ...any) (sql.Result, error) {
	result, err := q.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("db: raw exec: %w", err)
	}
	return result, nil
}

// RawQueryAll executes arbitrary SQL and scans all rows into []T using
// the same struct-tag scanner as the typed queries.
func RawQueryAll[T any](ctx context.Context, q Querier, query string, args ...any) ([]T, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("db: raw query: %w", err)
	}
	return scanAll[T](rows)
}

// RawQueryOne executes arbitrary SQL and scans a single row into T.
// Returns ErrNotFound if no rows match.
func RawQueryOne[T any](ctx context.Context, q Querier, query string, args ...any) (T, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("db: raw query: %w", err)
	}
	return scanOne[T](rows)
}

// RawCondition returns an Expr from raw SQL with optional args. Use it
// inside typed queries when the column API cannot express the condition.
//
//	db.Select(&Users.TableInfo).Where(db.RawCondition("length(name) > ?", 5))
func RawCondition(sql string, args ...any) Expr {
	return Raw(sql, args...)
}
