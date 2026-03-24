// Package driver provides a database/sql driver for SQLite, built directly
// from the C amalgamation source via CGo. Zero external Go dependencies.
//
// # Structured Errors
//
// All errors returned by the driver are of type *Error, which carries both the
// primary and extended SQLite result codes for programmatic error handling:
//
//	var sqlErr *driver.Error
//	if errors.As(err, &sqlErr) && sqlErr.Code == driver.CodeBusy {
//	    // handle busy
//	}
//
// # Context Cancellation
//
// ExecContext and QueryContext on both conn and stmt support context
// cancellation via sqlite3_interrupt(). When a context is cancelled, the
// running SQLite operation is interrupted and the context error is returned.
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
	"fmt"
	"io"
	"strings"
	"time"
	"unsafe"
)

// Compile-time interface assertions.
var (
	_ driver.Conn           = (*conn)(nil)
	_ driver.ExecerContext  = (*conn)(nil)
	_ driver.QueryerContext = (*conn)(nil)
	_ driver.Tx             = (*tx)(nil)
	_ driver.Stmt           = (*stmt)(nil)
	_ driver.StmtExecContext  = (*stmt)(nil)
	_ driver.StmtQueryContext = (*stmt)(nil)
	_ driver.Rows           = (*rows)(nil)
	_ driver.Result         = (*result)(nil)
)

// timeFormat is the canonical timestamp format for binding time.Time values.
// Matches SQLite's datetime() output with added millisecond precision.
// Always stored in UTC to avoid timezone ambiguity.
const timeFormat = "2006-01-02 15:04:05.000"

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

	// Enable extended result codes for this connection so that
	// sqlite3_extended_errcode() returns detailed sub-codes.
	C.sqlite3_extended_result_codes(db, 1)

	return &conn{db: db}, nil
}

// MemoryUsed returns the number of bytes of memory currently allocated by
// SQLite across all connections in this process.
func MemoryUsed() int64 {
	return int64(C.sqlite3_memory_used())
}

// MemoryHighwater returns the peak memory allocation since the process started
// or since the last highwater reset. Pass true to reset the counter.
func MemoryHighwater(reset bool) int64 {
	var r C.int
	if reset {
		r = 1
	}
	return int64(C.sqlite3_memory_highwater(r))
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
	if c.db == nil {
		return nil
	}
	rc := C.sqlite3_close_v2(c.db)
	if rc != C.SQLITE_OK {
		err := c.lastError()
		c.db = nil
		return err
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
// string (e.g., seed files with DELETE + INSERT). Supports context
// cancellation via sqlite3_interrupt().
func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if len(args) == 0 {
		cancel := c.watchCtx(ctx)
		err := c.exec(query)
		cancel()
		if err != nil {
			if isInterrupt(err) && ctx.Err() != nil {
				return nil, ctx.Err()
			}
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

	return s.(*stmt).ExecContext(ctx, args)
}

// QueryContext implements driver.QueryerContext. Prepares and executes the
// query, returning rows. The prepared statement is finalized when the rows
// are closed. Supports context cancellation via sqlite3_interrupt().
func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	s, err := c.Prepare(query)
	if err != nil {
		return nil, err
	}

	r, err := s.(*stmt).QueryContext(ctx, args)
	if err != nil {
		s.Close()
		return nil, err
	}

	// The rows object takes ownership of the stmt — it will be finalized
	// when the rows are closed.
	r.(*rows).closeStmt = true
	return r, nil
}

// lastError builds an *Error from the connection's last error state,
// including both the primary and extended result codes. The primary code
// is the lower 8 bits of the extended code.
func (c *conn) lastError() error {
	ext := int(C.sqlite3_extended_errcode(c.db))
	return &Error{
		Code:         ext & 0xFF,
		ExtendedCode: ext,
		Message:      C.GoString(C.sqlite3_errmsg(c.db)),
	}
}

func (c *conn) exec(query string) error {
	cSQL := C.CString(query)
	defer C.free(unsafe.Pointer(cSQL))
	rc := C.sqlite3_exec(c.db, cSQL, nil, nil, nil)
	if rc != C.SQLITE_OK {
		return c.lastError()
	}
	return nil
}

// watchCtx spawns a goroutine that calls sqlite3_interrupt when the context
// is cancelled. Returns a cleanup function that must be called when the
// operation completes (to stop the goroutine). Safe to call with a
// background context — returns a no-op.
func (c *conn) watchCtx(ctx context.Context) func() {
	if ctx.Done() == nil {
		return func() {} // background/TODO context
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			C.sqlite3_interrupt(c.db)
		case <-done:
		}
	}()
	return func() { close(done) }
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
	if s.s == nil {
		return nil
	}
	rc := C.sqlite3_finalize(s.s)
	if rc != C.SQLITE_OK {
		err := s.c.lastError()
		s.s = nil
		return err
	}
	s.s = nil
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

// ExecContext implements driver.StmtExecContext. Supports context cancellation
// via sqlite3_interrupt().
func (s *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if err := s.bind(namedToValues(args)); err != nil {
		return nil, err
	}

	cancel := s.c.watchCtx(ctx)
	rc := C.sqlite3_step(s.s)
	cancel()
	defer C.sqlite3_reset(s.s)
	defer C.sqlite3_clear_bindings(s.s)

	if rc == C.SQLITE_INTERRUPT {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
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

// QueryContext implements driver.StmtQueryContext. Supports context
// cancellation via sqlite3_interrupt() — the interrupt goroutine stays alive
// for the lifetime of the returned rows.
func (s *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := s.bind(namedToValues(args)); err != nil {
		return nil, err
	}

	numCols := int(C.sqlite3_column_count(s.s))
	cols := make([]string, numCols)
	for i := range numCols {
		cols[i] = C.GoString(C.sqlite3_column_name(s.s, C.int(i)))
	}

	cancel := s.c.watchCtx(ctx)
	return &rows{s: s, cols: cols, ctx: ctx, cancel: cancel}, nil
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
				// Copy to C-managed memory with C.free destructor — same
				// pattern as string binding. Using nil (SQLITE_STATIC) would
				// be unsound because the Go GC can collect the backing array
				// between bind() and the first sqlite3_step() call.
				p := C.CBytes(v)
				rc = C.sqlite3_bind_blob(s.s, idx, p, C.int(len(v)), (*[0]byte)(C.free))
			}
		case time.Time:
			formatted := v.UTC().Format(timeFormat)
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
	s         *stmt
	cols      []string
	ctx       context.Context // set by QueryContext for interrupt error reporting
	cancel    func()          // stops the watchCtx goroutine
	closeStmt bool            // true when rows owns the stmt (conn.QueryContext path)
}

func (r *rows) Columns() []string { return r.cols }

func (r *rows) Close() error {
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	C.sqlite3_reset(r.s.s)
	C.sqlite3_clear_bindings(r.s.s)
	if r.closeStmt {
		return r.s.Close()
	}
	return nil
}

func (r *rows) Next(dest []driver.Value) error {
	rc := C.sqlite3_step(r.s.s)
	if rc == C.SQLITE_DONE {
		return io.EOF
	}
	if rc == C.SQLITE_INTERRUPT {
		if r.ctx != nil && r.ctx.Err() != nil {
			return r.ctx.Err()
		}
		return r.s.c.lastError()
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

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// namedToValues converts NamedValue args to plain Values.
func namedToValues(args []driver.NamedValue) []driver.Value {
	values := make([]driver.Value, len(args))
	for i, a := range args {
		values[i] = a.Value
	}
	return values
}

// isInterrupt checks if an error is a SQLite SQLITE_INTERRUPT error.
func isInterrupt(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == CodeInterrupt
	}
	return false
}
