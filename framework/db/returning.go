package db

import (
	"context"
	"fmt"
	"strings"
)

// returnable is an unexported interface satisfied by InsertBuilder,
// UpdateBuilder, and DeleteBuilder. It allows the generic Returning and
// ReturningAll functions to accept any mutation builder.
type returnable interface {
	build() (string, []any, error)
}

// Returning executes a mutation query (INSERT, UPDATE, or DELETE) with a
// RETURNING clause and scans one row into T. Returns ErrNotFound if no rows
// are returned.
//
// The builder must have .Returning() called to specify which columns to return.
//
//	q := db.Insert(&Users.TableInfo).Model(&user).
//	    Returning(Users.ID, Users.Email, Users.Name, Users.CreatedAt, Users.UpdatedAt)
//	created, err := db.Returning[User](ctx, writeDB, q)
//
//	q := db.Update(&Users.TableInfo).Set(Users.Name, "New").
//	    Where(Users.ID.Eq(id)).
//	    Returning(Users.ID, Users.Name, Users.UpdatedAt)
//	updated, err := db.Returning[User](ctx, writeDB, q)
func Returning[T any](ctx context.Context, q Querier, b returnable) (T, error) {
	var zero T
	sqlStr, args, err := b.build()
	if err != nil {
		return zero, err
	}
	rows, err := q.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return zero, fmt.Errorf("db: returning: %w", err)
	}
	return scanOne[T](rows)
}

// ReturningAll executes a mutation query (INSERT, UPDATE, or DELETE) with a
// RETURNING clause and scans all returned rows into []T. Returns an empty
// slice (not nil) if no rows match.
//
//	q := db.Update(&Users.TableInfo).Set(Users.Active, false).
//	    Where(Users.Role.Eq("guest")).
//	    Returning(Users.ID, Users.Email)
//	deactivated, err := db.ReturningAll[User](ctx, writeDB, q)
func ReturningAll[T any](ctx context.Context, q Querier, b returnable) ([]T, error) {
	sqlStr, args, err := b.build()
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("db: returning: %w", err)
	}
	return scanAll[T](rows)
}

// writeReturning appends " RETURNING expr1, expr2, ..." to the buffer.
// Column types use unqualified names (just "col", not "table"."col") per
// SQLite RETURNING convention. Other expressions use WriteSQL as-is.
func writeReturning(buf *strings.Builder, args *[]any, exprs []Expr) {
	buf.WriteString(" RETURNING ")
	for i, expr := range exprs {
		if i > 0 {
			buf.WriteString(", ")
		}
		if col, ok := expr.(column); ok {
			buf.WriteString(quoteIdent(col.columnName()))
		} else {
			expr.WriteSQL(buf, args)
		}
	}
}
