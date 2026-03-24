package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// DeleteBuilder builds a DELETE query. Create one with Delete().
type DeleteBuilder struct {
	table     *TableInfo
	where     []Expr
	returning []Expr
	ctes      []*CTEDef
}

// With attaches Common Table Expressions to this DELETE. The WITH clause is
// rendered before the DELETE statement.
func (b *DeleteBuilder) With(ctes ...*CTEDef) *DeleteBuilder {
	b.ctes = append(b.ctes, ctes...)
	return b
}

// Delete starts a DELETE query for the given table.
func Delete(table *TableInfo) *DeleteBuilder {
	return &DeleteBuilder{table: table}
}

// Where appends WHERE conditions. Multiple calls are ANDed.
func (b *DeleteBuilder) Where(preds ...Expr) *DeleteBuilder {
	b.where = append(b.where, preds...)
	return b
}

// Build generates the SQL string and args.
func (b *DeleteBuilder) Build() (string, []any, error) {
	if len(b.where) == 0 {
		return "", nil, errors.New("db: DELETE without WHERE is not allowed (use Where(Raw(\"1=1\")) to delete all rows)")
	}

	var buf strings.Builder
	var args []any

	// WITH clause
	writeCTEs(&buf, &args, b.ctes)

	buf.WriteString("DELETE FROM ")
	buf.WriteString(quoteIdent(b.table.name))

	buf.WriteString(" WHERE ")
	writeExprs(&buf, &args, b.where)

	// RETURNING
	if len(b.returning) > 0 {
		writeReturning(&buf, &args, b.returning)
	}

	return buf.String(), args, nil
}

// Returning sets the columns or expressions to return from the DELETE. Use
// with the package-level Returning or ReturningAll functions to scan results.
//
//	q := db.Delete(&Users.TableInfo).Where(Users.ID.Eq(id)).
//	    Returning(Users.ID, Users.Email)
//	deleted, err := db.Returning[User](ctx, writeDB, q)
func (b *DeleteBuilder) Returning(exprs ...Expr) *DeleteBuilder {
	b.returning = exprs
	return b
}

// ReturningStar adds RETURNING * to the DELETE, returning all columns of
// each deleted row.
func (b *DeleteBuilder) ReturningStar() *DeleteBuilder {
	b.returning = []Expr{Raw("*")}
	return b
}

// build implements the returnable interface.
func (b *DeleteBuilder) build() (string, []any, error) {
	return b.Build()
}

// Exec executes the DELETE and returns the result.
// Returns an error if no WHERE clause is set (safety guard).
func (b *DeleteBuilder) Exec(ctx context.Context, q Querier) (sql.Result, error) {
	sqlStr, args, err := b.Build()
	if err != nil {
		return nil, err
	}
	result, err := q.ExecContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("db: delete: %w", err)
	}
	return result, nil
}
