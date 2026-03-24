package db

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// autoFillBaseModel sets empty ID and zero timestamps on a struct's BaseModel
// fields via reflection. Called by Model() and ModelSlice() to auto-populate
// standard fields on insertion.
func autoFillBaseModel(rv reflect.Value, m *fieldMapping, now time.Time) {
	if idIdx, ok := m.colToIndex["id"]; ok {
		f := rv.FieldByIndex(idIdx)
		if f.CanSet() && f.Kind() == reflect.String && f.String() == "" {
			f.SetString(NewID())
		}
	}
	for _, col := range []string{"created_at", "updated_at"} {
		if idx, ok := m.colToIndex[col]; ok {
			f := rv.FieldByIndex(idx)
			if f.CanSet() && f.Type() == reflect.TypeOf(time.Time{}) && f.Interface().(time.Time).IsZero() {
				f.Set(reflect.ValueOf(now))
			}
		}
	}
}

// InsertBuilder builds an INSERT query. Create one with Insert().
type InsertBuilder struct {
	table     *TableInfo
	columns   []column
	values    [][]any // one inner slice per row
	conflict  *conflictClause
	returning []column
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
//   - Skips nil pointer fields (lets DB DEFAULT apply)
//   - Dereferences non-nil pointer fields before binding
//
// Columns are emitted in deterministic (sorted) order.
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

	// Auto-fill BaseModel fields when a pointer is passed.
	if canSet {
		autoFillBaseModel(rv, m, now)
	}

	sorted := sortedMapKeys(m.colToIndex)
	var cols []column
	var vals []any

	for _, colName := range sorted {
		idx := m.colToIndex[colName]
		field := rv.FieldByIndex(idx)
		fieldType := field.Type()
		val := field.Interface()

		// Pointer types: skip nil (let DB DEFAULT apply), dereference non-nil.
		if fieldType.Kind() == reflect.Ptr {
			if field.IsNil() {
				continue
			}
			cols = append(cols, newSyntheticColumn(b.table.name, colName))
			vals = append(vals, field.Elem().Interface())
			continue
		}

		// Non-pointer auto-fill (for the case where canSet is false).
		if colName == "id" && fieldType.Kind() == reflect.String && val.(string) == "" {
			cols = append(cols, newSyntheticColumn(b.table.name, colName))
			vals = append(vals, NewID())
			continue
		}
		if (colName == "created_at" || colName == "updated_at") &&
			fieldType == reflect.TypeOf(time.Time{}) && val.(time.Time).IsZero() {
			cols = append(cols, newSyntheticColumn(b.table.name, colName))
			vals = append(vals, now)
			continue
		}

		cols = append(cols, newSyntheticColumn(b.table.name, colName))
		vals = append(vals, val)
	}

	b.columns = cols
	b.values = [][]any{vals}
	return b
}

// ModelSlice reads a slice of structs and builds a multi-row INSERT.
// Each element is processed like Model(): BaseModel fields (ID, timestamps)
// are auto-filled. Unlike Model(), nil pointer fields insert NULL rather
// than being omitted, since all rows must have identical column lists.
//
// Accepts []T or []*T. For pointer slices, nil elements are skipped.
//
//	notes := []Note{{Title: "A"}, {Title: "B"}, {Title: "C"}}
//	db.Insert(&Notes.TableInfo).ModelSlice(notes).Exec(ctx, writeDB)
func (b *InsertBuilder) ModelSlice(slice any) *InsertBuilder {
	rv := reflect.ValueOf(slice)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice || rv.Len() == 0 {
		return b
	}

	elemType := rv.Type().Elem()
	isPtr := elemType.Kind() == reflect.Ptr
	if isPtr {
		elemType = elemType.Elem()
	}

	m := getMapping(elemType)
	now := time.Now()
	sorted := sortedMapKeys(m.colToIndex)

	// Build column list (same for all rows).
	cols := make([]column, len(sorted))
	for i, k := range sorted {
		cols[i] = newSyntheticColumn(b.table.name, k)
	}

	// Extract values for each row.
	allVals := make([][]any, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		if isPtr {
			if elem.IsNil() {
				continue
			}
			elem = elem.Elem()
		}

		// Auto-fill BaseModel fields.
		autoFillBaseModel(elem, m, now)

		vals := make([]any, len(sorted))
		for j, colName := range sorted {
			idx := m.colToIndex[colName]
			field := elem.FieldByIndex(idx)
			fieldType := field.Type()

			if fieldType.Kind() == reflect.Ptr {
				if field.IsNil() {
					vals[j] = nil
				} else {
					vals[j] = field.Elem().Interface()
				}
				continue
			}

			val := field.Interface()
			if colName == "id" && fieldType.Kind() == reflect.String && val.(string) == "" {
				vals[j] = NewID()
			} else if (colName == "created_at" || colName == "updated_at") &&
				fieldType == reflect.TypeOf(time.Time{}) && val.(time.Time).IsZero() {
				vals[j] = now
			} else {
				vals[j] = val
			}
		}
		allVals = append(allVals, vals)
	}

	b.columns = cols
	b.values = allVals
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

	// RETURNING
	if len(b.returning) > 0 {
		writeReturning(&buf, b.returning)
	}

	return buf.String(), args
}

// Returning sets the columns to return from the INSERT. Use with the
// package-level Returning or ReturningAll functions to scan the results.
//
//	q := db.Insert(&Users.TableInfo).Model(&user).Returning(Users.ID, Users.Email)
//	created, err := db.Returning[User](ctx, writeDB, q)
func (b *InsertBuilder) Returning(cols ...column) *InsertBuilder {
	b.returning = cols
	return b
}

// build implements the returnable interface.
func (b *InsertBuilder) build() (string, []any, error) {
	sql, args := b.Build()
	return sql, args, nil
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
