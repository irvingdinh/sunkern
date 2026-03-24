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
	table      *TableInfo
	columns    []column
	values     [][]any // one inner slice per row
	fromSelect Query   // when set, renders INSERT...SELECT instead of VALUES
	conflict   *conflictClause
	returning  []Expr
	ctes       []*CTEDef
}

// With attaches Common Table Expressions to this INSERT. The WITH clause is
// rendered before the INSERT statement.
func (b *InsertBuilder) With(ctes ...*CTEDef) *InsertBuilder {
	b.ctes = append(b.ctes, ctes...)
	return b
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

// FromSelect sets a SELECT query as the data source, producing INSERT...SELECT
// instead of INSERT...VALUES. Columns must be set via Columns() to define the
// target column list. Model() and Values() are ignored when FromSelect is used.
//
//	db.Insert(&Archive.TableInfo).
//	    Columns(Archive.ID, Archive.Title, Archive.CreatedAt).
//	    FromSelect(
//	        db.Select(&Posts.TableInfo).
//	            Columns(Posts.ID, Posts.Title, Posts.CreatedAt).
//	            Where(Posts.Status.Eq("archived")),
//	    ).
//	    Exec(ctx, writeDB)
//
// INSERT...SELECT composes with ON CONFLICT and RETURNING:
//
//	db.Insert(&Archive.TableInfo).
//	    Columns(Archive.ID, Archive.Title).
//	    FromSelect(selectQuery).
//	    OnConflict(Archive.ID).DoNothing().
//	    Exec(ctx, writeDB)
func (b *InsertBuilder) FromSelect(query Query) *InsertBuilder {
	b.fromSelect = query
	return b
}

// OnConflict begins an ON CONFLICT clause targeting the given columns.
// Call DoNothing(), DoUpdate(), or DoUpdateAll() on the returned
// ConflictBuilder to complete it.
//
//	db.Insert(&Settings.TableInfo).
//	    Model(setting).
//	    OnConflict(Settings.Key).
//	    DoUpdate(db.SetExcluded(Settings.Value)).
//	    Exec(ctx, writeDB)
func (b *InsertBuilder) OnConflict(cols ...column) *ConflictBuilder {
	return &ConflictBuilder{insert: b, targets: cols}
}

// ConflictWhere adds a WHERE condition to the DO UPDATE clause of an
// ON CONFLICT. The update is only applied when the condition is true.
// Must be called after OnConflict().DoUpdate(). Use with Excluded() to
// compare incoming vs existing values.
//
//	db.Insert(&Settings.TableInfo).
//	    Model(setting).
//	    OnConflict(Settings.Key).
//	    DoUpdate(db.SetExcluded(Settings.Value), db.SetExcluded(Settings.Version)).
//	    ConflictWhere(db.ColGt(db.Excluded(Settings.Version), Settings.Version))
func (b *InsertBuilder) ConflictWhere(preds ...Expr) *InsertBuilder {
	if b.conflict != nil {
		b.conflict.updateWhere = append(b.conflict.updateWhere, preds...)
	}
	return b
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

	// WITH clause
	writeCTEs(&buf, &args, b.ctes)

	buf.WriteString("INSERT INTO ")
	buf.WriteString(quoteIdent(b.table.name))

	// Columns (optional for INSERT...SELECT without explicit columns)
	if len(b.columns) > 0 {
		buf.WriteString(" (")
		for i, col := range b.columns {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(quoteIdent(col.columnName()))
		}
		buf.WriteString(")")
	}

	if b.fromSelect != nil {
		// INSERT...SELECT
		buf.WriteString(" ")
		selectSQL, selectArgs := b.fromSelect.Build()
		buf.WriteString(selectSQL)
		args = append(args, selectArgs...)
	} else {
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
	}

	// ON CONFLICT
	if b.conflict != nil {
		b.conflict.writeConflict(&buf, &args)
	}

	// RETURNING
	if len(b.returning) > 0 {
		writeReturning(&buf, &args, b.returning)
	}

	return buf.String(), args
}

// Returning sets the columns or expressions to return from the INSERT. Use
// with the package-level Returning or ReturningAll functions to scan results.
// Accepts typed columns (Users.ID), aliases (As(Users.ID, "uid")), and
// raw expressions (Raw("datetime('now')")).
//
//	q := db.Insert(&Users.TableInfo).Model(&user).Returning(Users.ID, Users.Email)
//	created, err := db.Returning[User](ctx, writeDB, q)
func (b *InsertBuilder) Returning(exprs ...Expr) *InsertBuilder {
	b.returning = exprs
	return b
}

// ReturningStar adds RETURNING * to the INSERT, returning all columns of the
// inserted row.
func (b *InsertBuilder) ReturningStar() *InsertBuilder {
	b.returning = []Expr{Raw("*")}
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
