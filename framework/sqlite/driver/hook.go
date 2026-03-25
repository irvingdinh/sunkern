package driver

// Preamble: declarations only (required by //export).

/*
#include "sqlite3.h"
#include <stdlib.h>

// Install functions defined in hook.c.
extern void sunkern_install_trace(sqlite3 *db, unsigned int mask, void *ctx);
extern void sunkern_install_busy(sqlite3 *db, void *ctx);
extern void sunkern_install_wal(sqlite3 *db, void *ctx);
extern void sunkern_install_update(sqlite3 *db, void *ctx);
*/
import "C"

import (
	"runtime/cgo"
	"time"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Public Types
// ---------------------------------------------------------------------------

// TraceMask controls which trace events are reported to a [TraceFunc].
type TraceMask uint

const (
	// TraceStmt fires when a prepared statement first starts executing.
	// [TraceInfo].SQL contains the original (unexpanded) SQL text.
	TraceStmt TraceMask = 0x01

	// TraceProfile fires when a prepared statement finishes executing.
	// [TraceInfo].SQL contains the expanded SQL (parameters substituted)
	// and [TraceInfo].Duration contains the wall-clock execution time.
	TraceProfile TraceMask = 0x02
)

// TraceInfo describes a single SQL trace event.
type TraceInfo struct {
	// EventType is TraceStmt or TraceProfile.
	EventType TraceMask
	// SQL is the SQL text. For TraceStmt this is the original text; for
	// TraceProfile this is the expanded text with bound parameter values.
	SQL string
	// Duration is the statement execution time (TraceProfile only; zero
	// for TraceStmt).
	Duration time.Duration
}

// TraceFunc is called for each SQL trace event. It runs synchronously in the
// goroutine executing the SQL statement — keep the callback fast to avoid
// adding latency to every query.
type TraceFunc func(TraceInfo)

// BusyFunc is called when the database is locked by another connection.
// count is the number of times the handler has been invoked for the current
// locking event (starting from 0). Return true to retry, false to return
// SQLITE_BUSY to the caller.
//
// Setting a BusyFunc on a [Connector] overrides PRAGMA busy_timeout for
// connections created through that connector.
type BusyFunc func(count int) bool

// WALFunc is called after a transaction commits in WAL mode. pages is the
// number of frames in the WAL file after the commit. It runs synchronously
// in the committing goroutine — keep it fast.
type WALFunc func(dbName string, pages int)

// UpdateAction identifies the type of row change that triggered an update hook.
type UpdateAction int

const (
	// ActionInsert indicates a row was inserted.
	ActionInsert UpdateAction = 18 // SQLITE_INSERT
	// ActionDelete indicates a row was deleted.
	ActionDelete UpdateAction = 9 // SQLITE_DELETE
	// ActionUpdate indicates a row was updated.
	ActionUpdate UpdateAction = 23 // SQLITE_UPDATE
)

// String returns a human-readable label for the action ("insert", "delete",
// or "update").
func (a UpdateAction) String() string {
	switch a {
	case ActionInsert:
		return "insert"
	case ActionDelete:
		return "delete"
	case ActionUpdate:
		return "update"
	default:
		return "unknown"
	}
}

// UpdateInfo describes a single row change event.
type UpdateInfo struct {
	// Action is the type of change (insert, delete, or update).
	Action UpdateAction
	// DBName is the database name (usually "main").
	DBName string
	// Table is the name of the table that changed.
	Table string
	// RowID is the ROWID of the affected row. For WITHOUT ROWID tables,
	// this value is undefined.
	RowID int64
}

// UpdateFunc is called for every row INSERT, UPDATE, or DELETE on the
// connection. It runs synchronously in the goroutine executing the statement
// — keep it fast to avoid adding latency to every write.
//
// The callback fires before the statement completes, so the row is already
// visible within the same transaction but may not be committed yet.
type UpdateFunc func(UpdateInfo)

// DefaultBusyHandler returns a [BusyFunc] implementing exponential backoff.
// Delays start at 1ms and double each retry up to 50ms, then stay flat.
// Retries stop after approximately maxWait total sleep time.
func DefaultBusyHandler(maxWait time.Duration) BusyFunc {
	// Precompute max retries from maxWait.
	// Ramp phase (count 0–5): delays 1+2+4+8+16+32 = 63ms.
	// Flat phase (count 6+): 50ms per retry.
	maxRetries := 6
	if maxWait > 63*time.Millisecond {
		maxRetries += int((maxWait - 63*time.Millisecond) / (50 * time.Millisecond))
	}
	return func(count int) bool {
		if count >= maxRetries {
			return false
		}
		delay := time.Millisecond << uint(min(count, 5))
		if delay > 50*time.Millisecond {
			delay = 50 * time.Millisecond
		}
		time.Sleep(delay)
		return true
	}
}

// ---------------------------------------------------------------------------
// Connector hook methods
// ---------------------------------------------------------------------------

// SetTrace registers a trace callback that fires for every SQL statement
// executed on connections created by this Connector. Pass nil to disable.
// mask controls which events are reported (TraceStmt, TraceProfile, or both).
//
// Only affects future connections. Existing pooled connections are unchanged.
func (c *Connector) SetTrace(fn TraceFunc, mask TraceMask) {
	c.mu.Lock()
	c.trace = fn
	c.traceMask = mask
	c.mu.Unlock()
}

// SetBusyHandler registers a busy handler for connections created by this
// Connector. Pass nil to disable (reverts to PRAGMA busy_timeout behavior).
//
// Only affects future connections. Existing pooled connections are unchanged.
func (c *Connector) SetBusyHandler(fn BusyFunc) {
	c.mu.Lock()
	c.busy = fn
	c.mu.Unlock()
}

// SetWALHook registers a WAL commit hook for connections created by this
// Connector. Pass nil to disable.
//
// Only affects future connections. Existing pooled connections are unchanged.
func (c *Connector) SetWALHook(fn WALFunc) {
	c.mu.Lock()
	c.wal = fn
	c.mu.Unlock()
}

// SetUpdateHook registers a row-change notification hook for connections
// created by this Connector. The callback fires for every INSERT, UPDATE,
// and DELETE on the connection. Pass nil to disable.
//
// Only affects future connections. Existing pooled connections are unchanged.
func (c *Connector) SetUpdateHook(fn UpdateFunc) {
	c.mu.Lock()
	c.update = fn
	c.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Internal: hook handle and installation
// ---------------------------------------------------------------------------

// connHooks holds the Go callbacks for a single database connection. Stored
// via cgo.Handle so C callbacks can retrieve them safely.
type connHooks struct {
	trace  TraceFunc
	busy   BusyFunc
	wal    WALFunc
	update UpdateFunc
}

// installHooks registers C-level callbacks on the connection, using handle
// as the context pointer passed through to the Go callbacks.
func (cn *conn) installHooks(h *connHooks, mask TraceMask, handle cgo.Handle) {
	ctx := unsafe.Pointer(handle)

	if h.trace != nil {
		C.sunkern_install_trace(cn.db, C.uint(mask), ctx)
	}
	if h.busy != nil {
		C.sunkern_install_busy(cn.db, ctx)
	}
	if h.wal != nil {
		C.sunkern_install_wal(cn.db, ctx)
	}
	if h.update != nil {
		C.sunkern_install_update(cn.db, ctx)
	}
}

// ---------------------------------------------------------------------------
// CGo exported callbacks
// ---------------------------------------------------------------------------

//export sunkernTraceCallback
func sunkernTraceCallback(mask C.uint, ctx unsafe.Pointer, p unsafe.Pointer, x unsafe.Pointer) C.int {
	h := cgo.Handle(ctx)
	hooks, ok := h.Value().(*connHooks)
	if !ok || hooks.trace == nil {
		return 0
	}

	info := TraceInfo{EventType: TraceMask(mask)}

	switch TraceMask(mask) {
	case TraceStmt:
		// x is the original SQL text (const char*).
		if x != nil {
			info.SQL = C.GoString((*C.char)(x))
		}
	case TraceProfile:
		// p is sqlite3_stmt*; get expanded SQL (parameters substituted).
		stmt := (*C.sqlite3_stmt)(p)
		expanded := C.sqlite3_expanded_sql(stmt)
		if expanded != nil {
			info.SQL = C.GoString(expanded)
			C.sqlite3_free(unsafe.Pointer(expanded))
		} else {
			// Fallback to unexpanded SQL if expansion fails (e.g., BLOB params).
			orig := C.sqlite3_sql(stmt)
			if orig != nil {
				info.SQL = C.GoString(orig)
			}
		}
		// x is sqlite3_int64* pointing to nanosecond duration.
		if x != nil {
			ns := *(*C.sqlite3_int64)(x)
			info.Duration = time.Duration(ns)
		}
	}

	hooks.trace(info)
	return 0
}

//export sunkernBusyCallback
func sunkernBusyCallback(ctx unsafe.Pointer, count C.int) C.int {
	h := cgo.Handle(ctx)
	hooks, ok := h.Value().(*connHooks)
	if !ok || hooks.busy == nil {
		return 0 // give up
	}
	if hooks.busy(int(count)) {
		return 1 // retry
	}
	return 0 // give up → SQLITE_BUSY
}

//export sunkernWALCallback
func sunkernWALCallback(ctx unsafe.Pointer, _ unsafe.Pointer, name *C.char, pages C.int) C.int {
	h := cgo.Handle(ctx)
	hooks, ok := h.Value().(*connHooks)
	if !ok || hooks.wal == nil {
		return 0
	}
	hooks.wal(C.GoString(name), int(pages))
	return 0
}

//export sunkernUpdateCallback
func sunkernUpdateCallback(ctx unsafe.Pointer, action C.int, db *C.char, table *C.char, rowid C.sqlite3_int64) {
	h := cgo.Handle(ctx)
	hooks, ok := h.Value().(*connHooks)
	if !ok || hooks.update == nil {
		return
	}
	hooks.update(UpdateInfo{
		Action: UpdateAction(action),
		DBName: C.GoString(db),
		Table:  C.GoString(table),
		RowID:  int64(rowid),
	})
}
