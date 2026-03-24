// Package driver provides a database/sql driver for SQLite, built directly
// from the C amalgamation source via CGo. Zero external Go dependencies.
//
// Import this package for its side effect of registering the "sqlite3" driver:
//
//	import _ "sunkern.local/framework/sqlite/driver"
package driver

/*
#cgo CFLAGS: -std=gnu99
#cgo CFLAGS: -DSQLITE_THREADSAFE=1
#cgo CFLAGS: -DSQLITE_ENABLE_FTS5
#cgo CFLAGS: -DSQLITE_ENABLE_JSON1
#cgo CFLAGS: -DSQLITE_ENABLE_RTREE
#cgo CFLAGS: -DSQLITE_DQS=0
#cgo CFLAGS: -DSQLITE_DEFAULT_WAL_SYNCHRONOUS=1
#cgo CFLAGS: -DSQLITE_DEFAULT_FOREIGN_KEYS=1
#cgo CFLAGS: -DSQLITE_ENABLE_UPDATE_DELETE_LIMIT
#cgo linux LDFLAGS: -lm -ldl -lpthread
#cgo darwin CFLAGS: -DHAVE_USLEEP=1

#include "sqlite3.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unsafe"
)

func init() {
	if rc := C.sqlite3_initialize(); rc != C.SQLITE_OK {
		panic(fmt.Sprintf("sqlite3: initialize failed: error code %d", rc))
	}
	sql.Register("sqlite3", &Driver{})
}

// Driver implements database/sql/driver.Driver.
type Driver struct{}

// Open opens a new SQLite connection. The DSN is a URI-style path:
//
//	file:/path/to/db             (read-write, create)
//	file:/path/to/db?mode=ro     (read-only)
//	:memory:                     (in-memory)
func (d *Driver) Open(dsn string) (driver.Conn, error) {
	flags := C.SQLITE_OPEN_READWRITE | C.SQLITE_OPEN_CREATE |
		C.SQLITE_OPEN_FULLMUTEX | C.SQLITE_OPEN_URI

	if strings.Contains(dsn, "mode=ro") {
		flags = C.SQLITE_OPEN_READONLY | C.SQLITE_OPEN_FULLMUTEX | C.SQLITE_OPEN_URI
	}

	cDSN := C.CString(dsn)
	defer C.free(unsafe.Pointer(cDSN))

	var db *C.sqlite3
	rc := C.sqlite3_open_v2(cDSN, &db, C.int(flags), nil)
	if rc != C.SQLITE_OK {
		if db != nil {
			msg := C.GoString(C.sqlite3_errmsg(db))
			C.sqlite3_close_v2(db)
			return nil, fmt.Errorf("sqlite3: open %q: %s", dsn, msg)
		}
		return nil, fmt.Errorf("sqlite3: open %q: error code %d", dsn, rc)
	}

	return &conn{db: db}, nil
}

// ---------------------------------------------------------------------------
// conn
// ---------------------------------------------------------------------------

type conn struct {
	db *C.sqlite3
}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	cSQL := C.CString(query)
	defer C.free(unsafe.Pointer(cSQL))

	var s *C.sqlite3_stmt
	rc := C.sqlite3_prepare_v2(c.db, cSQL, C.int(len(query)), &s, nil)
	if rc != C.SQLITE_OK {
		return nil, c.lastError()
	}

	return &stmt{c: c, s: s}, nil
}

func (c *conn) Close() error {
	rc := C.sqlite3_close_v2(c.db)
	if rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite3: close: error code %d", rc)
	}
	c.db = nil
	return nil
}

func (c *conn) Begin() (driver.Tx, error) {
	if err := c.exec("BEGIN"); err != nil {
		return nil, err
	}
	return &tx{c: c}, nil
}

// ExecContext implements driver.ExecerContext. When there are no parameters,
// it uses sqlite3_exec which handles multiple SQL statements in a single
// string (e.g., seed files with DELETE + INSERT).
func (c *conn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if len(args) == 0 {
		if err := c.exec(query); err != nil {
			return nil, err
		}
		return &result{
			changes: int64(C.sqlite3_changes(c.db)),
			lastID:  int64(C.sqlite3_last_insert_rowid(c.db)),
		}, nil
	}

	// With parameters, fall through to prepare+exec.
	s, err := c.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	values := make([]driver.Value, len(args))
	for i, a := range args {
		values[i] = a.Value
	}
	return s.(driver.Stmt).Exec(values)
}

func (c *conn) lastError() error {
	return errors.New("sqlite3: " + C.GoString(C.sqlite3_errmsg(c.db)))
}

func (c *conn) exec(sql string) error {
	cSQL := C.CString(sql)
	defer C.free(unsafe.Pointer(cSQL))
	rc := C.sqlite3_exec(c.db, cSQL, nil, nil, nil)
	if rc != C.SQLITE_OK {
		return c.lastError()
	}
	return nil
}

// ---------------------------------------------------------------------------
// tx
// ---------------------------------------------------------------------------

type tx struct {
	c *conn
}

func (t *tx) Commit() error   { return t.c.exec("COMMIT") }
func (t *tx) Rollback() error { return t.c.exec("ROLLBACK") }

// ---------------------------------------------------------------------------
// stmt
// ---------------------------------------------------------------------------

type stmt struct {
	c *conn
	s *C.sqlite3_stmt
}

func (s *stmt) Close() error {
	rc := C.sqlite3_finalize(s.s)
	s.s = nil
	if rc != C.SQLITE_OK {
		return s.c.lastError()
	}
	return nil
}

func (s *stmt) NumInput() int {
	return int(C.sqlite3_bind_parameter_count(s.s))
}

func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	if err := s.bind(args); err != nil {
		return nil, err
	}

	rc := C.sqlite3_step(s.s)
	defer C.sqlite3_reset(s.s)
	defer C.sqlite3_clear_bindings(s.s)

	if rc != C.SQLITE_DONE && rc != C.SQLITE_ROW {
		return nil, s.c.lastError()
	}

	return &result{
		changes: int64(C.sqlite3_changes(s.c.db)),
		lastID:  int64(C.sqlite3_last_insert_rowid(s.c.db)),
	}, nil
}

func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	if err := s.bind(args); err != nil {
		return nil, err
	}

	numCols := int(C.sqlite3_column_count(s.s))
	cols := make([]string, numCols)
	for i := range numCols {
		cols[i] = C.GoString(C.sqlite3_column_name(s.s, C.int(i)))
	}

	return &rows{s: s, cols: cols}, nil
}

func (s *stmt) bind(args []driver.Value) error {
	C.sqlite3_reset(s.s)
	C.sqlite3_clear_bindings(s.s)

	for i, arg := range args {
		idx := C.int(i + 1)
		var rc C.int

		switch v := arg.(type) {
		case nil:
			rc = C.sqlite3_bind_null(s.s, idx)
		case int64:
			rc = C.sqlite3_bind_int64(s.s, idx, C.sqlite3_int64(v))
		case float64:
			rc = C.sqlite3_bind_double(s.s, idx, C.double(v))
		case bool:
			if v {
				rc = C.sqlite3_bind_int64(s.s, idx, 1)
			} else {
				rc = C.sqlite3_bind_int64(s.s, idx, 0)
			}
		case string:
			cStr := C.CString(v)
			rc = C.sqlite3_bind_text(s.s, idx, cStr, C.int(len(v)), (*[0]byte)(C.free))
		case []byte:
			if len(v) == 0 {
				rc = C.sqlite3_bind_zeroblob(s.s, idx, 0)
			} else {
				rc = C.sqlite3_bind_blob(s.s, idx, unsafe.Pointer(&v[0]), C.int(len(v)), nil)
			}
		case time.Time:
			formatted := v.Format("2006-01-02 15:04:05")
			cStr := C.CString(formatted)
			rc = C.sqlite3_bind_text(s.s, idx, cStr, C.int(len(formatted)), (*[0]byte)(C.free))
		default:
			return fmt.Errorf("sqlite3: unsupported bind type %T at index %d", arg, i)
		}

		if rc != C.SQLITE_OK {
			return s.c.lastError()
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// rows
// ---------------------------------------------------------------------------

type rows struct {
	s    *stmt
	cols []string
}

func (r *rows) Columns() []string { return r.cols }

func (r *rows) Close() error {
	C.sqlite3_reset(r.s.s)
	C.sqlite3_clear_bindings(r.s.s)
	return nil
}

func (r *rows) Next(dest []driver.Value) error {
	rc := C.sqlite3_step(r.s.s)
	if rc == C.SQLITE_DONE {
		return io.EOF
	}
	if rc != C.SQLITE_ROW {
		return r.s.c.lastError()
	}

	for i := range dest {
		ci := C.int(i)
		switch C.sqlite3_column_type(r.s.s, ci) {
		case C.SQLITE_NULL:
			dest[i] = nil
		case C.SQLITE_INTEGER:
			dest[i] = int64(C.sqlite3_column_int64(r.s.s, ci))
		case C.SQLITE_FLOAT:
			dest[i] = float64(C.sqlite3_column_double(r.s.s, ci))
		case C.SQLITE_TEXT:
			n := C.sqlite3_column_bytes(r.s.s, ci)
			p := C.sqlite3_column_text(r.s.s, ci)
			dest[i] = C.GoStringN((*C.char)(unsafe.Pointer(p)), n)
		case C.SQLITE_BLOB:
			n := C.sqlite3_column_bytes(r.s.s, ci)
			p := C.sqlite3_column_blob(r.s.s, ci)
			if n == 0 {
				dest[i] = []byte{}
			} else {
				dest[i] = C.GoBytes(p, n)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// result
// ---------------------------------------------------------------------------

type result struct {
	changes int64
	lastID  int64
}

func (r *result) LastInsertId() (int64, error) { return r.lastID, nil }
func (r *result) RowsAffected() (int64, error) { return r.changes, nil }
