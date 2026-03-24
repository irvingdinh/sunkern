package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// SelectBuilder builds a SELECT query. Create one with Select().
type SelectBuilder struct {
	table    *TableInfo
	columns  []Expr // if empty, uses table.Star()
	distinct bool
	where    []Expr // ANDed together
	orderBy  []OrderExpr
	limit    *int
	offset   *int
	joins    []joinClause
	groupBy  []Expr
	having   []Expr
}

type joinClause struct {
	joinType string // "JOIN" or "LEFT JOIN"
	table    *TableInfo
	on       Expr
}

// Select starts a SELECT query for the given table.
func Select(table *TableInfo) *SelectBuilder {
	return &SelectBuilder{table: table}
}

// Distinct causes SELECT DISTINCT to be generated.
func (b *SelectBuilder) Distinct() *SelectBuilder {
	b.distinct = true
	return b
}

// Columns sets the explicit column list. If not called, all columns from
// the table's Star() are selected.
func (b *SelectBuilder) Columns(cols ...Expr) *SelectBuilder {
	b.columns = append(b.columns, cols...)
	return b
}

// Where appends WHERE conditions. Multiple calls are ANDed.
func (b *SelectBuilder) Where(preds ...Expr) *SelectBuilder {
	b.where = append(b.where, preds...)
	return b
}

// OrderBy appends ORDER BY expressions.
func (b *SelectBuilder) OrderBy(exprs ...OrderExpr) *SelectBuilder {
	b.orderBy = append(b.orderBy, exprs...)
	return b
}

// Limit sets the LIMIT clause.
func (b *SelectBuilder) Limit(n int) *SelectBuilder {
	b.limit = &n
	return b
}

// Offset sets the OFFSET clause.
func (b *SelectBuilder) Offset(n int) *SelectBuilder {
	b.offset = &n
	return b
}

// Join adds an INNER JOIN.
func (b *SelectBuilder) Join(table *TableInfo, on Expr) *SelectBuilder {
	b.joins = append(b.joins, joinClause{joinType: "JOIN", table: table, on: on})
	return b
}

// LeftJoin adds a LEFT JOIN.
func (b *SelectBuilder) LeftJoin(table *TableInfo, on Expr) *SelectBuilder {
	b.joins = append(b.joins, joinClause{joinType: "LEFT JOIN", table: table, on: on})
	return b
}

// GroupBy sets the GROUP BY columns.
func (b *SelectBuilder) GroupBy(cols ...Expr) *SelectBuilder {
	b.groupBy = append(b.groupBy, cols...)
	return b
}

// Having appends HAVING conditions. Multiple calls are ANDed.
func (b *SelectBuilder) Having(preds ...Expr) *SelectBuilder {
	b.having = append(b.having, preds...)
	return b
}

// Apply applies scopes (reusable query modifiers) to the builder.
func (b *SelectBuilder) Apply(scopes ...Scope) *SelectBuilder {
	for _, s := range scopes {
		b = s(b)
	}
	return b
}

// Build generates the SQL string and args without executing.
func (b *SelectBuilder) Build() (string, []any) {
	var buf strings.Builder
	var args []any

	// SELECT
	buf.WriteString("SELECT ")
	if b.distinct {
		buf.WriteString("DISTINCT ")
	}
	cols := b.columns
	if len(cols) == 0 {
		cols = b.table.Star()
	}
	for i, col := range cols {
		if i > 0 {
			buf.WriteString(", ")
		}
		col.WriteSQL(&buf, &args)
	}

	// FROM
	buf.WriteString(" FROM ")
	b.table.WriteSQL(&buf, &args)

	// JOINs
	for _, j := range b.joins {
		buf.WriteString(" ")
		buf.WriteString(j.joinType)
		buf.WriteString(" ")
		j.table.WriteSQL(&buf, &args)
		buf.WriteString(" ON ")
		j.on.WriteSQL(&buf, &args)
	}

	// WHERE
	if len(b.where) > 0 {
		buf.WriteString(" WHERE ")
		writeExprs(&buf, &args, b.where)
	}

	// GROUP BY
	if len(b.groupBy) > 0 {
		buf.WriteString(" GROUP BY ")
		for i, col := range b.groupBy {
			if i > 0 {
				buf.WriteString(", ")
			}
			col.WriteSQL(&buf, &args)
		}
	}

	// HAVING
	if len(b.having) > 0 {
		buf.WriteString(" HAVING ")
		writeExprs(&buf, &args, b.having)
	}

	// ORDER BY
	if len(b.orderBy) > 0 {
		buf.WriteString(" ORDER BY ")
		for i, o := range b.orderBy {
			if i > 0 {
				buf.WriteString(", ")
			}
			o.WriteSQL(&buf, &args)
		}
	}

	// LIMIT
	if b.limit != nil {
		buf.WriteString(" LIMIT ")
		buf.WriteString(strconv.Itoa(*b.limit))
	}

	// OFFSET
	if b.offset != nil {
		buf.WriteString(" OFFSET ")
		buf.WriteString(strconv.Itoa(*b.offset))
	}

	return buf.String(), args
}

// buildCount generates a SELECT COUNT(*) query reusing FROM/WHERE/JOIN.
func (b *SelectBuilder) buildCount() (string, []any) {
	var buf strings.Builder
	var args []any

	if b.distinct && len(b.columns) > 0 {
		buf.WriteString("SELECT COUNT(DISTINCT ")
		for i, col := range b.columns {
			if i > 0 {
				buf.WriteString(", ")
			}
			col.WriteSQL(&buf, &args)
		}
		buf.WriteString(") FROM ")
	} else {
		buf.WriteString("SELECT COUNT(*) FROM ")
	}
	b.table.WriteSQL(&buf, &args)

	for _, j := range b.joins {
		buf.WriteString(" ")
		buf.WriteString(j.joinType)
		buf.WriteString(" ")
		j.table.WriteSQL(&buf, &args)
		buf.WriteString(" ON ")
		j.on.WriteSQL(&buf, &args)
	}

	if len(b.where) > 0 {
		buf.WriteString(" WHERE ")
		writeExprs(&buf, &args, b.where)
	}

	if len(b.groupBy) > 0 {
		buf.WriteString(" GROUP BY ")
		for i, col := range b.groupBy {
			if i > 0 {
				buf.WriteString(", ")
			}
			col.WriteSQL(&buf, &args)
		}
	}

	if len(b.having) > 0 {
		buf.WriteString(" HAVING ")
		writeExprs(&buf, &args, b.having)
	}

	return buf.String(), args
}

// ---------------------------------------------------------------------------
// Generic free functions (type-safe scan)
// ---------------------------------------------------------------------------

// QueryAll executes the select query and scans all rows into []T.
func QueryAll[T any](ctx context.Context, q Querier, sb *SelectBuilder) ([]T, error) {
	sql, args := sb.Build()
	rows, err := q.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("db: query: %w", err)
	}
	return scanAll[T](rows)
}

// QueryOne executes the select query and scans a single row into T.
// Returns ErrNotFound if no rows match.
func QueryOne[T any](ctx context.Context, q Querier, sb *SelectBuilder) (T, error) {
	sql, args := sb.Build()
	rows, err := q.QueryContext(ctx, sql, args...)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("db: query: %w", err)
	}
	return scanOne[T](rows)
}

// QueryVal executes the query and scans a single column from the first row
// into T. Use for scalar results (e.g., MAX, single-column selects).
// Returns ErrNotFound if no rows match.
//
//	maxAge, err := db.QueryVal[int64](ctx, readDB,
//	    db.Select(&Users.TableInfo).Columns(db.Max(Users.Age, "")))
func QueryVal[T any](ctx context.Context, q Querier, sb *SelectBuilder) (T, error) {
	sqlStr, args := sb.Build()
	rows, err := q.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("db: query: %w", err)
	}
	return scanVal[T](rows)
}

// Count executes a SELECT COUNT(*) using the builder's FROM/WHERE/JOIN clauses.
func Count(ctx context.Context, q Querier, sb *SelectBuilder) (int64, error) {
	sql, args := sb.buildCount()
	rows, err := q.QueryContext(ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("db: count: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return 0, fmt.Errorf("db: count: no rows returned")
	}
	var count int64
	if err := rows.Scan(&count); err != nil {
		return 0, fmt.Errorf("db: count scan: %w", err)
	}
	return count, nil
}

// Exists returns true if at least one row matches the query.
func Exists(ctx context.Context, q Querier, sb *SelectBuilder) (bool, error) {
	count, err := Count(ctx, q, sb)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
