package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// UpdateBuilder builds an UPDATE query. Create one with Update().
type UpdateBuilder struct {
	table *TableInfo
	sets  []setClause
	where []Expr
}

type setClause struct {
	col column
	val any
}

// Update starts an UPDATE query for the given table.
func Update(table *TableInfo) *UpdateBuilder {
	return &UpdateBuilder{table: table}
}

// Set adds a "column = value" assignment.
func (b *UpdateBuilder) Set(col column, val any) *UpdateBuilder {
	b.sets = append(b.sets, setClause{col: col, val: val})
	return b
}

// Where appends WHERE conditions. Multiple calls are ANDed.
func (b *UpdateBuilder) Where(preds ...Expr) *UpdateBuilder {
	b.where = append(b.where, preds...)
	return b
}

// Build generates the SQL string and args.
func (b *UpdateBuilder) Build() (string, []any, error) {
	if len(b.where) == 0 {
		return "", nil, errors.New("db: UPDATE without WHERE is not allowed (use Where(Raw(\"1=1\")) to update all rows)")
	}
	if len(b.sets) == 0 {
		return "", nil, errors.New("db: UPDATE with no SET clauses")
	}

	var buf strings.Builder
	var args []any

	buf.WriteString("UPDATE ")
	buf.WriteString(quoteIdent(b.table.name))

	// SET
	buf.WriteString(" SET ")
	for i, s := range b.sets {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(quoteIdent(s.col.columnName()))
		buf.WriteString(" = ?")
		args = append(args, s.val)
	}

	// WHERE
	buf.WriteString(" WHERE ")
	writeExprs(&buf, &args, b.where)

	return buf.String(), args, nil
}

// Exec executes the UPDATE and returns the result.
// Returns an error if no WHERE clause is set (safety guard).
func (b *UpdateBuilder) Exec(ctx context.Context, q Querier) (sql.Result, error) {
	sqlStr, args, err := b.Build()
	if err != nil {
		return nil, err
	}
	result, err := q.ExecContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("db: update: %w", err)
	}
	return result, nil
}
