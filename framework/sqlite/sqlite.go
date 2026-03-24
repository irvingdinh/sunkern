package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	_ "sunkern.local/framework/sqlite/driver"

	"sunkern.local/framework/config"
	"sunkern.local/framework/container"
)

// DB holds the dual read/write connection pools to the SQLite database.
// The write pool has MaxOpenConns=1 to serialize all writes and eliminate
// SQLITE_BUSY errors. The read pool has MaxOpenConns matching GOMAXPROCS
// for concurrent reads.
type DB struct {
	write *sql.DB
	read  *sql.DB
	path  string
}

// WriteDB returns the write-only connection pool (MaxOpenConns=1).
func (db *DB) WriteDB() *sql.DB { return db.write }

// ReadDB returns the read-only connection pool.
func (db *DB) ReadDB() *sql.DB { return db.read }

// Path returns the database file path.
func (db *DB) Path() string { return db.path }

var global struct {
	mu sync.Mutex
	db *DB
}

// Load opens the SQLite database, applies PRAGMAs, and registers a shutdown
// hook to close both pools. It reads the database path from config
// ({data_dir}/database.sqlite). Call after config.Load().
func Load() {
	dataDir := config.Get[string]("data_dir")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		panic(fmt.Sprintf("sqlite: creating data directory: %v", err))
	}

	dbPath := filepath.Join(dataDir, "database.sqlite")

	writeDB, err := sql.Open("sqlite3", "file:"+dbPath)
	if err != nil {
		panic(fmt.Sprintf("sqlite: open write pool: %v", err))
	}
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)

	readConns := runtime.GOMAXPROCS(0)
	if readConns < 4 {
		readConns = 4
	}
	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		writeDB.Close()
		panic(fmt.Sprintf("sqlite: open read pool: %v", err))
	}
	readDB.SetMaxOpenConns(readConns)
	readDB.SetMaxIdleConns(readConns)

	// Apply PRAGMAs.
	if err := applyPragmas(writeDB, readDB); err != nil {
		writeDB.Close()
		readDB.Close()
		panic(fmt.Sprintf("sqlite: apply pragmas: %v", err))
	}

	// Verify connectivity.
	if err := writeDB.Ping(); err != nil {
		writeDB.Close()
		readDB.Close()
		panic(fmt.Sprintf("sqlite: ping write pool: %v", err))
	}

	global.mu.Lock()
	global.db = &DB{write: writeDB, read: readDB, path: dbPath}
	global.mu.Unlock()

	container.AppendHook(container.Hook{
		OnStop: func(_ context.Context) error {
			return Close()
		},
	})

	slog.Info("sqlite: database opened", "path", dbPath, "read_conns", readConns)
}

// Global returns the global DB instance. Panics if Load has not been called.
func Global() *DB {
	global.mu.Lock()
	defer global.mu.Unlock()
	if global.db == nil {
		panic("sqlite: Load has not been called")
	}
	return global.db
}

// Close closes both the read and write pools. Safe to call multiple times.
func Close() error {
	global.mu.Lock()
	defer global.mu.Unlock()
	if global.db == nil {
		return nil
	}
	writeErr := global.db.write.Close()
	readErr := global.db.read.Close()
	global.db = nil
	return errors.Join(writeErr, readErr)
}

// Reset closes the database and clears the global state. Intended for tests.
func Reset() {
	_ = Close()
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func applyPragmas(write, read *sql.DB) error {
	// PRAGMAs for both pools.
	shared := []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -16000",
		"PRAGMA foreign_keys = ON",
		"PRAGMA temp_store = MEMORY",
	}

	// PRAGMAs only for the write pool.
	writeOnly := []string{
		"PRAGMA journal_size_limit = 67108864",
		"PRAGMA wal_autocheckpoint = 1000",
	}

	for _, pool := range []*sql.DB{write, read} {
		for _, pragma := range shared {
			if _, err := pool.Exec(pragma); err != nil {
				return fmt.Errorf("%s: %w", pragma, err)
			}
		}
	}

	for _, pragma := range writeOnly {
		if _, err := write.Exec(pragma); err != nil {
			return fmt.Errorf("%s: %w", pragma, err)
		}
	}

	return nil
}
