package db

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// InsertBuilder builds an INSERT query. Create one with Insert().
type InsertBuilder struct {
	table    *TableInfo
	columns  []column
	values   [][]any // one inner slice per row
	conflict *conflictClause
}

// Insert starts an INSERT query for the given table.
func Insert(table *TableInfo) *InsertBuilder {
	return &InsertBuilder{table: table}
}

// Columns sets the target columns. Must be called before Values.
func (b *InsertBuilder) Columns(cols ...column) *InsertBuilder {
	b.columns = cols
	return b
}

// Values adds one row of values. The count must match the columns set by
// Columns. Call multiple times for batch insert.
func (b *InsertBuilder) Values(vals ...any) *InsertBuilder {
	b.values = append(b.values, vals)
	return b
}

// OnConflict begins an ON CONFLICT clause targeting the given columns.
// Call DoNothing() or DoUpdate() on the returned ConflictBuilder to complete it.
//
//	db.Insert(&Settings.TableInfo).
//	    Model(setting).
//	    OnConflict(Settings.Key).
//	    DoUpdate(db.SetExcluded(Settings.Value)).
//	    Exec(ctx, writeDB)
func (b *InsertBuilder) OnConflict(cols ...column) *ConflictBuilder {
	return &ConflictBuilder{insert: b, targets: cols}
}

// Model reads a struct's db tags to determine columns and values. When
// a pointer is passed, it auto-fills empty BaseModel fields so the caller's
// struct matches what gets inserted:
//   - Generates a new ID if the "id" field is an empty string
//   - Sets created_at and updated_at to time.Now() if they are zero values
//   - Skips nil *time.Time fields (lets DB DEFAULT apply)
//
// Usage:
//
//	user := User{BaseModel: db.NewBaseModel(), Email: "a@b.com"}
//	db.Insert(&Users.TableInfo).Model(&user).Exec(ctx, writeDB)
//
// Or let Model auto-fill:
//
//	user := User{Email: "a@b.com"}
//	db.Insert(&Users.TableInfo).Model(&user).Exec(ctx, writeDB)
//	// user.ID, user.CreatedAt, user.UpdatedAt are now set
func (b *InsertBuilder) Model(v any) *InsertBuilder {
	rv := reflect.ValueOf(v)
	canSet := rv.Kind() == reflect.Ptr
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	rt := rv.Type()

	m := getMapping(rt)
	now := time.Now()

	// Auto-fill BaseModel fields when a pointer is passed, so the caller's
	// struct stays in sync with what gets inserted.
	if canSet {
		if idIdx, ok := m.colToIndex["id"]; ok {
			f := rv.FieldByIndex(idIdx)
			if f.Kind() == reflect.String && f.String() == "" {
				f.SetString(NewID())
			}
		}
		for _, col := range []string{"created_at", "updated_at"} {
			if idx, ok := m.colToIndex[col]; ok {
				f := rv.FieldByIndex(idx)
				if f.Type() == reflect.TypeOf(time.Time{}) && f.Interface().(time.Time).IsZero() {
					f.Set(reflect.ValueOf(now))
				}
			}
		}
	}

	var cols []column
	var vals []any

	for colName, idx := range m.colToIndex {
		field := rv.FieldByIndex(idx)
		fieldType := field.Type()
		val := field.Interface()

		// Handle *time.Time: skip nil (let DB DEFAULT apply).
		if fieldType == reflect.TypeOf((*time.Time)(nil)) {
			if field.IsNil() {
				continue
			}
			t := val.(*time.Time)
			cols = append(cols, newSyntheticColumn(b.table.name, colName))
			vals = append(vals, *t)
			continue
		}

		// Handle time.Time: auto-set created_at/updated_at if zero.
		// This covers the non-pointer path where canSet is false.
		if fieldType == reflect.TypeOf(time.Time{}) {
			t := val.(time.Time)
			if t.IsZero() && (colName == "created_at" || colName == "updated_at") {
				t = now
			}
			cols = append(cols, newSyntheticColumn(b.table.name, colName))
			vals = append(vals, t)
			continue
		}

		// Auto-generate ID for non-pointer path.
		if colName == "id" && fieldType.Kind() == reflect.String && val.(string) == "" {
			cols = append(cols, newSyntheticColumn(b.table.name, colName))
			vals = append(vals, NewID())
			continue
		}

		cols = append(cols, newSyntheticColumn(b.table.name, colName))
		vals = append(vals, val)
	}

	b.columns = cols
	b.values = [][]any{vals}
	return b
}

// Build generates the SQL string and args.
func (b *InsertBuilder) Build() (string, []any) {
	var buf strings.Builder
	var args []any

	buf.WriteString("INSERT INTO ")
	buf.WriteString(quoteIdent(b.table.name))

	// Columns
	buf.WriteString(" (")
	for i, col := range b.columns {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(quoteIdent(col.columnName()))
	}
	buf.WriteString(")")

	// VALUES
	buf.WriteString(" VALUES ")
	for ri, row := range b.values {
		if ri > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString("(")
		for vi, val := range row {
			if vi > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString("?")
			args = append(args, val)
		}
		buf.WriteString(")")
	}

	// ON CONFLICT
	if b.conflict != nil {
		b.conflict.writeConflict(&buf, &args)
	}

	return buf.String(), args
}

// Exec executes the INSERT and returns the result.
func (b *InsertBuilder) Exec(ctx context.Context, q Querier) (sql.Result, error) {
	sqlStr, args := b.Build()
	result, err := q.ExecContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("db: insert: %w", err)
	}
	return result, nil
}

// syntheticColumn is a minimal column implementation used by Model() when
// it discovers columns via reflection rather than from the typed column vars.
type syntheticColumn struct {
	columnRef
}

func newSyntheticColumn(table, name string) syntheticColumn {
	return syntheticColumn{columnRef: columnRef{table: table, name: name}}
}
