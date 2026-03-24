package driver

import "fmt"

// Error represents a SQLite error with both the primary and extended result
// codes. Use errors.As to extract it from wrapped errors:
//
//	var sqlErr *driver.Error
//	if errors.As(err, &sqlErr) {
//	    switch sqlErr.Code {
//	    case driver.CodeBusy:
//	        // retry logic
//	    case driver.CodeConstraint:
//	        // duplicate or FK violation
//	    }
//	}
type Error struct {
	// Code is the primary result code (e.g., CodeBusy = 5).
	Code int
	// ExtendedCode is the full result code including sub-code
	// (e.g., CodeConstraintUnique = 2067).
	ExtendedCode int
	// Message is the English error message from sqlite3_errmsg().
	Message string
}

func (e *Error) Error() string {
	if e.ExtendedCode != 0 && e.ExtendedCode != e.Code {
		return fmt.Sprintf("sqlite3: %s (%d/%d)", e.Message, e.Code, e.ExtendedCode)
	}
	return fmt.Sprintf("sqlite3: %s (%d)", e.Message, e.Code)
}

// Primary result codes. These correspond to the SQLITE_* constants from the
// C API. Only the codes relevant to application-level error handling are
// exported — the full list is in the SQLite documentation.
const (
	CodeOK         = 0  // SQLITE_OK — not an error
	CodeError      = 1  // SQLITE_ERROR — generic error
	CodeBusy       = 5  // SQLITE_BUSY — database is locked
	CodeLocked     = 6  // SQLITE_LOCKED — table is locked
	CodeNoMem      = 7  // SQLITE_NOMEM — out of memory
	CodeReadOnly   = 8  // SQLITE_READONLY — attempt to write a readonly database
	CodeInterrupt  = 9  // SQLITE_INTERRUPT — operation interrupted by sqlite3_interrupt
	CodeFull       = 13 // SQLITE_FULL — database or disk is full
	CodeAuth       = 23 // SQLITE_AUTH — authorization denied
	CodeConstraint = 19 // SQLITE_CONSTRAINT — constraint violation
	CodeMismatch   = 20 // SQLITE_MISMATCH — datatype mismatch
)

// Extended result codes. These provide finer-grained error information by
// combining the primary code with a sub-code in the upper bits.
const (
	// Busy sub-codes.
	CodeBusyRecovery = 5 | (1 << 8) // 261 — WAL recovery in progress
	CodeBusySnapshot = 5 | (2 << 8) // 517 — snapshot conflict in WAL mode
	CodeBusyTimeout  = 5 | (3 << 8) // 773 — busy_timeout expired

	// Locked sub-codes.
	CodeLockedSharedCache = 6 | (1 << 8) // 262 — shared cache contention

	// Constraint sub-codes.
	CodeConstraintCheck      = 19 | (1 << 8) // 275 — CHECK constraint failed
	CodeConstraintCommitHook = 19 | (2 << 8) // 531 — commit hook caused rollback
	CodeConstraintForeignKey = 19 | (3 << 8) // 787 — FOREIGN KEY constraint failed
	CodeConstraintNotNull    = 19 | (5 << 8) // 1299 — NOT NULL constraint failed
	CodeConstraintPrimaryKey = 19 | (6 << 8) // 1555 — PRIMARY KEY constraint failed
	CodeConstraintTrigger    = 19 | (7 << 8) // 1811 — RAISE in trigger
	CodeConstraintUnique     = 19 | (8 << 8) // 2067 — UNIQUE constraint failed
	CodeConstraintRowID      = 19 | (10 << 8) // 2579 — rowid constraint

	// ReadOnly sub-codes.
	CodeReadOnlyRecovery = 8 | (1 << 8) // 264 — WAL recovery needed, read-only
	CodeReadOnlyDBMoved  = 8 | (4 << 8) // 1032 — database file moved
)
