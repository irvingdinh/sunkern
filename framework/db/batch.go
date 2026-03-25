package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// BatchUpdateBuilder builds a single UPDATE statement that sets different
// values per row using CASE expressions. This is far more efficient than
// issuing N individual UPDATE queries for bulk operations.
//
// All rows are matched by a single key column (typically the ID column).
// Each row specifies its own SET values. Columns not set on a specific row
// retain their existing database value.
//
// Generated SQL:
//
//	UPDATE "users" SET
//	  "name" = CASE "id" WHEN ? THEN ? WHEN ? THEN ? ELSE "name" END,
//	  "email" = CASE "id" WHEN ? THEN ? WHEN ? THEN ? ELSE "email" END,
//	  "updated_at" = ?
//	WHERE "id" IN (?, ?)
//
// Usage:
//
//	batch := db.BatchUpdate(&Users.TableInfo, Users.ID)
//	batch.Row("abc").Set(Users.Name, "Alice").Set(Users.Email, "alice@x.com")
//	batch.Row("def").Set(Users.Name, "Bob")
//	result, err := batch.Exec(ctx, writeDB)
type BatchUpdateBuilder struct {
	table  *TableInfo
	keyCol column
	rows   []*BatchRow
}

// BatchRow holds the SET assignments for a single row in a batch update.
type BatchRow struct {
	batch  *BatchUpdateBuilder
	keyVal any
	sets   []batchSet
}

type batchSet struct {
	col column
	val any
}

// BatchUpdate starts a batch update for the given table, keyed by the
// specified column. Call Row() to add per-row assignments.
func BatchUpdate(table *TableInfo, keyCol column) *BatchUpdateBuilder {
	return &BatchUpdateBuilder{table: table, keyCol: keyCol}
}

// Row adds a row to the batch identified by the given key value.
// Chain .Set() calls on the returned BatchRow to specify column assignments.
func (b *BatchUpdateBuilder) Row(keyVal any) *BatchRow {
	r := &BatchRow{batch: b, keyVal: keyVal}
	b.rows = append(b.rows, r)
	return r
}

// Set adds a column assignment for this row.
func (r *BatchRow) Set(col column, val any) *BatchRow {
	r.sets = append(r.sets, batchSet{col: col, val: val})
	return r
}

// Row is a convenience to chain back to the parent builder for adding another
// row without breaking the fluent chain.
func (r *BatchRow) Row(keyVal any) *BatchRow {
	return r.batch.Row(keyVal)
}

// Build generates the SQL string and args. Returns an error if no rows
// are present or no SET clauses are defined across all rows.
func (b *BatchUpdateBuilder) Build() (string, []any, error) {
	if len(b.rows) == 0 {
		return "", nil, errors.New("db: batch update with no rows")
	}

	// Collect the union of all columns across all rows, preserving
	// deterministic order (first-seen order per column name).
	type colInfo struct {
		col   column
		order int
	}
	colMap := make(map[string]colInfo)
	var colOrder []string

	for _, row := range b.rows {
		for _, s := range row.sets {
			name := s.col.columnName()
			if _, exists := colMap[name]; !exists {
				colMap[name] = colInfo{col: s.col, order: len(colOrder)}
				colOrder = append(colOrder, name)
			}
		}
	}

	if len(colOrder) == 0 {
		return "", nil, errors.New("db: batch update with no SET clauses")
	}

	var buf strings.Builder
	var args []any

	buf.WriteString("UPDATE ")
	buf.WriteString(quoteIdent(b.table.name))
	buf.WriteString(" SET ")

	keyColName := b.keyCol.columnName()

	// Build a CASE expression per column.
	for ci, colName := range colOrder {
		if ci > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(quoteIdent(colName))
		buf.WriteString(" = CASE ")
		b.keyCol.WriteSQL(&buf, &args)

		for _, row := range b.rows {
			val, ok := rowVal(row, colName)
			if !ok {
				continue
			}
			buf.WriteString(" WHEN ? THEN ?")
			args = append(args, row.keyVal, val)
		}

		// ELSE preserves existing value for rows that don't set this column.
		buf.WriteString(" ELSE ")
		buf.WriteString(quoteIdent(colName))
		buf.WriteString(" END")
	}

	// Auto-set updated_at if not already in the column set.
	if _, hasUpdatedAt := colMap["updated_at"]; !hasUpdatedAt {
		buf.WriteString(", ")
		buf.WriteString(quoteIdent("updated_at"))
		buf.WriteString(" = ?")
		args = append(args, time.Now())
	}

	// WHERE key_col IN (val1, val2, ...)
	buf.WriteString(" WHERE ")
	buf.WriteString(quoteIdent(keyColName))
	buf.WriteString(" IN (")
	for i, row := range b.rows {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString("?")
		args = append(args, row.keyVal)
	}
	buf.WriteString(")")

	return buf.String(), args, nil
}

// Exec executes the batch UPDATE and returns the result.
func (b *BatchUpdateBuilder) Exec(ctx context.Context, q Querier) (sql.Result, error) {
	sqlStr, args, err := b.Build()
	if err != nil {
		return nil, err
	}
	result, err := q.ExecContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("db: batch update: %w", err)
	}
	return result, nil
}

// rowVal finds the value for a given column name in a BatchRow.
func rowVal(row *BatchRow, colName string) (any, bool) {
	for _, s := range row.sets {
		if s.col.columnName() == colName {
			return s.val, true
		}
	}
	return nil, false
}
