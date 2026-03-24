package db

import (
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Struct-tag mapping cache
// ---------------------------------------------------------------------------

// fieldMapping holds the cached mapping from column name → struct field index
// path (supporting embedded structs).
type fieldMapping struct {
	// colToIndex maps column name (from db tag) to the field's index chain.
	// For example, a field at BaseModel.ID has index [0, 0] (first field of
	// first embedded struct).
	colToIndex map[string][]int
}

var mappingCache sync.Map // map[reflect.Type]*fieldMapping

// getMapping returns the cached field mapping for the given struct type.
func getMapping(t reflect.Type) *fieldMapping {
	if v, ok := mappingCache.Load(t); ok {
		return v.(*fieldMapping)
	}

	m := &fieldMapping{colToIndex: make(map[string][]int)}
	buildMapping(t, nil, m)
	mappingCache.Store(t, m)
	return m
}

// buildMapping recursively walks struct fields, following embedded structs.
func buildMapping(t reflect.Type, prefix []int, m *fieldMapping) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		// Skip unexported fields.
		if !f.IsExported() {
			continue
		}

		idx := append(append([]int{}, prefix...), i)

		// Follow embedded structs.
		if f.Anonymous {
			ft := f.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				buildMapping(ft, idx, m)
				continue
			}
		}

		tag := f.Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}

		m.colToIndex[tag] = idx
	}
}

// ---------------------------------------------------------------------------
// scanAll / scanOne — generic row scanning
// ---------------------------------------------------------------------------

// scanAll scans all rows into a slice of T. Closes rows when done.
func scanAll[T any](rows *sql.Rows) ([]T, error) {
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("db: columns: %w", err)
	}

	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	m := getMapping(t)

	var result []T
	for rows.Next() {
		item, err := scanRow[T](rows, columns, m)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: rows iteration: %w", err)
	}
	if result == nil {
		result = make([]T, 0)
	}
	return result, nil
}

// scanVal scans a single column from the first row into T. Use for scalar
// queries (COUNT, MAX, single column selects). Returns ErrNotFound if no rows.
// Closes rows when done.
func scanVal[T any](rows *sql.Rows) (T, error) {
	defer rows.Close()

	var zero T
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero, fmt.Errorf("db: rows: %w", err)
		}
		return zero, ErrNotFound
	}

	var val T
	if err := rows.Scan(&val); err != nil {
		return zero, fmt.Errorf("db: scan: %w", err)
	}
	return val, nil
}

// scanOne scans exactly one row into T. Returns ErrNotFound if no rows.
// Closes rows when done.
func scanOne[T any](rows *sql.Rows) (T, error) {
	defer rows.Close()

	var zero T
	columns, err := rows.Columns()
	if err != nil {
		return zero, fmt.Errorf("db: columns: %w", err)
	}

	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	m := getMapping(t)

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero, fmt.Errorf("db: rows: %w", err)
		}
		return zero, ErrNotFound
	}

	return scanRow[T](rows, columns, m)
}

// scanRow scans a single row using the column-to-field mapping.
func scanRow[T any](rows *sql.Rows, columns []string, m *fieldMapping) (T, error) {
	var item T
	v := reflect.ValueOf(&item).Elem()

	// Build scan destinations. Use any-typed intermediaries so we can
	// post-process types that database/sql doesn't handle natively
	// (like string → time.Time).
	dests := make([]any, len(columns))
	for i := range dests {
		dests[i] = new(any)
	}

	if err := rows.Scan(dests...); err != nil {
		return item, fmt.Errorf("db: scan: %w", err)
	}

	// Map scanned values to struct fields.
	for i, colName := range columns {
		idx, ok := m.colToIndex[colName]
		if !ok {
			continue // column not mapped to any struct field
		}

		rawVal := *(dests[i].(*any))
		field := v.FieldByIndex(idx)
		if err := setField(field, rawVal); err != nil {
			return item, fmt.Errorf("db: set field %q: %w", colName, err)
		}
	}

	return item, nil
}

// sortedMapKeys returns the keys of a map[string][]int in sorted order.
// Used by Model/SetModel for deterministic column ordering in generated SQL.
func sortedMapKeys(m map[string][]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// setField converts a raw driver value to the target struct field type.
// Handles the key conversions: string→time.Time, nil→pointer, nil→zero.
func setField(field reflect.Value, rawVal any) error {
	if rawVal == nil {
		// NULL handling: set pointer types to nil, non-pointers to zero.
		if field.Kind() == reflect.Ptr {
			field.Set(reflect.Zero(field.Type()))
		}
		// Non-pointer zero value is the default; nothing to do.
		return nil
	}

	fieldType := field.Type()

	// Handle pointer fields: allocate and set inner value.
	if fieldType.Kind() == reflect.Ptr {
		elemType := fieldType.Elem()
		ptr := reflect.New(elemType)
		if err := setField(ptr.Elem(), rawVal); err != nil {
			return err
		}
		field.Set(ptr)
		return nil
	}

	// bool fields: SQLite stores booleans as INTEGER 0/1.
	if fieldType.Kind() == reflect.Bool {
		switch v := rawVal.(type) {
		case int64:
			field.SetBool(v != 0)
			return nil
		case bool:
			field.SetBool(v)
			return nil
		}
	}

	// time.Time fields: the driver returns TEXT as string, parse it.
	if fieldType == reflect.TypeOf(time.Time{}) {
		switch v := rawVal.(type) {
		case string:
			t, err := time.Parse(timeFormat, v)
			if err != nil {
				// Try with milliseconds (driver writes "2006-01-02 15:04:05.000").
				t, err = time.Parse(timeFormatMs, v)
				if err != nil {
					// Try RFC3339 as final fallback.
					t, err = time.Parse(time.RFC3339, v)
					if err != nil {
						return fmt.Errorf("parse time %q: %w", v, err)
					}
				}
			}
			field.Set(reflect.ValueOf(t))
			return nil
		case time.Time:
			field.Set(reflect.ValueOf(v))
			return nil
		}
	}

	// Standard conversions.
	rv := reflect.ValueOf(rawVal)
	if rv.Type().AssignableTo(fieldType) {
		field.Set(rv)
		return nil
	}
	if rv.Type().ConvertibleTo(fieldType) {
		field.Set(rv.Convert(fieldType))
		return nil
	}

	return fmt.Errorf("cannot convert %T to %s", rawVal, fieldType)
}
