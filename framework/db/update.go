package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// UpdateBuilder builds an UPDATE query. Create one with Update().
type UpdateBuilder struct {
	table     *TableInfo
	sets      []setClause
	where     []Expr
	returning []column
}

type setClause struct {
	col  column
	val  any  // used when expr is nil
	expr Expr // when non-nil, takes precedence over val
}

// Update starts an UPDATE query for the given table.
func Update(table *TableInfo) *UpdateBuilder {
	return &UpdateBuilder{table: table}
}

// Set adds a "column = value" assignment with a parameterized value.
func (b *UpdateBuilder) Set(col column, val any) *UpdateBuilder {
	b.sets = append(b.sets, setClause{col: col, val: val})
	return b
}

// SetExpr adds a "column = <expression>" assignment where the expression is
// any Expr. Use with Raw() for arithmetic:
//
//	db.Update(&Jobs.TableInfo).
//	    SetExpr(Jobs.Attempts, db.Raw(`"jobs"."attempts" + ?`, 1)).
//	    Where(Jobs.ID.Eq(jobID))
func (b *UpdateBuilder) SetExpr(col column, expr Expr) *UpdateBuilder {
	b.sets = append(b.sets, setClause{col: col, expr: expr})
	return b
}

// SetNull adds "column = NULL" to the SET clause. Use this to explicitly
// set a nullable column to NULL, since SetModel() skips nil pointers
// (which means "don't update this field").
//
//	db.Update(&Notes.TableInfo).
//	    SetModel(req).
//	    SetNull(Notes.Description).
//	    Where(Notes.ID.Eq(id))
func (b *UpdateBuilder) SetNull(col column) *UpdateBuilder {
	return b.SetExpr(col, Raw("NULL"))
}

// Increment adds "column = column + amount" to the SET clause.
func (b *UpdateBuilder) Increment(col column, amount int64) *UpdateBuilder {
	return b.SetExpr(col, Raw(
		quoteIdent(b.table.name)+"."+quoteIdent(col.columnName())+" + ?", amount,
	))
}

// Decrement adds "column = column - amount" to the SET clause.
func (b *UpdateBuilder) Decrement(col column, amount int64) *UpdateBuilder {
	return b.SetExpr(col, Raw(
		quoteIdent(b.table.name)+"."+quoteIdent(col.columnName())+" - ?", amount,
	))
}

// SetModel reads a struct's db tags and adds SET clauses for all non-zero
// fields. It:
//   - Always sets updated_at to time.Now()
//   - Skips "id" and "created_at" (never update these)
//   - Skips nil pointer fields (preserves existing DB value)
//   - Dereferences non-nil pointer fields before binding
//   - Skips zero-value non-pointer fields (zero = "don't update this field")
//
// SET clauses are emitted in deterministic (sorted) order.
//
// For explicit zero-value updates, chain .Set() after .SetModel().
// For explicit NULL updates, chain .SetNull() after .SetModel().
func (b *UpdateBuilder) SetModel(v any) *UpdateBuilder {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	rt := rv.Type()

	m := getMapping(rt)
	now := time.Now()

	for _, colName := range sortedMapKeys(m.colToIndex) {
		// Never update primary key or creation timestamp.
		if colName == "id" || colName == "created_at" {
			continue
		}

		idx := m.colToIndex[colName]
		field := rv.FieldByIndex(idx)
		fieldType := field.Type()

		// Always set updated_at to now.
		if colName == "updated_at" {
			col := newSyntheticColumn(b.table.name, colName)
			b.sets = append(b.sets, setClause{col: col, val: now})
			continue
		}

		// Pointer types: skip nil (preserves DB value), dereference non-nil.
		if fieldType.Kind() == reflect.Ptr {
			if field.IsNil() {
				continue
			}
			col := newSyntheticColumn(b.table.name, colName)
			b.sets = append(b.sets, setClause{col: col, val: field.Elem().Interface()})
			continue
		}

		// Skip zero-value non-pointer fields.
		if field.IsZero() {
			continue
		}

		col := newSyntheticColumn(b.table.name, colName)
		b.sets = append(b.sets, setClause{col: col, val: field.Interface()})
	}

	return b
}

// Where appends WHERE conditions. Multiple calls are ANDed.
func (b *UpdateBuilder) Where(preds ...Expr) *UpdateBuilder {
	b.where = append(b.where, preds...)
	return b
}

// Build generates the SQL string and args. If no SET clause targets
// "updated_at", one is auto-appended with the current time. This ensures
// models that embed BaseModel always get their timestamp refreshed.
func (b *UpdateBuilder) Build() (string, []any, error) {
	if len(b.where) == 0 {
		return "", nil, errors.New("db: UPDATE without WHERE is not allowed (use Where(Raw(\"1=1\")) to update all rows)")
	}
	if len(b.sets) == 0 {
		return "", nil, errors.New("db: UPDATE with no SET clauses")
	}

	// Auto-set updated_at if not already present.
	hasUpdatedAt := false
	for _, s := range b.sets {
		if s.col.columnName() == "updated_at" {
			hasUpdatedAt = true
			break
		}
	}
	if !hasUpdatedAt {
		b.sets = append(b.sets, setClause{
			col: newSyntheticColumn(b.table.name, "updated_at"),
			val: time.Now(),
		})
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
		buf.WriteString(" = ")
		if s.expr != nil {
			s.expr.WriteSQL(&buf, &args)
		} else {
			buf.WriteString("?")
			args = append(args, s.val)
		}
	}

	// WHERE
	buf.WriteString(" WHERE ")
	writeExprs(&buf, &args, b.where)

	// RETURNING
	if len(b.returning) > 0 {
		writeReturning(&buf, b.returning)
	}

	return buf.String(), args, nil
}

// Returning sets the columns to return from the UPDATE. Use with the
// package-level Returning or ReturningAll functions to scan the results.
//
//	q := db.Update(&Users.TableInfo).Set(Users.Name, "New").
//	    Where(Users.ID.Eq(id)).Returning(Users.ID, Users.Name, Users.UpdatedAt)
//	updated, err := db.Returning[User](ctx, writeDB, q)
func (b *UpdateBuilder) Returning(cols ...column) *UpdateBuilder {
	b.returning = cols
	return b
}

// build implements the returnable interface.
func (b *UpdateBuilder) build() (string, []any, error) {
	return b.Build()
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
