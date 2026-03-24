package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
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
//
// Configurable PRAGMAs (via config or env vars):
//
//	db.busy_timeout       — ms to wait for locks (default 5000, env DB_BUSY_TIMEOUT)
//	db.cache_size         — page cache size, negative = KB (default -16000, env DB_CACHE_SIZE)
//	db.mmap_size          — memory-mapped I/O limit in bytes (default 268435456 = 256MB, env DB_MMAP_SIZE)
//	db.wal_autocheckpoint — auto-checkpoint threshold in pages (default 1000, env DB_WAL_AUTOCHECKPOINT)
//	db.journal_size_limit — max WAL size after checkpoint in bytes (default 67108864 = 64MB, env DB_JOURNAL_SIZE_LIMIT)
func Load() {
	dbPath := filepath.Join(config.DataDir(), "database.sqlite")

	// Register config defaults so they are discoverable via config.Keys()
	// and config.All() (e.g., for an admin settings page).
	config.SetDefault("db.busy_timeout", 5000)
	config.SetDefault("db.cache_size", -16000)
	config.SetDefault("db.mmap_size", 268435456)
	config.SetDefault("db.wal_autocheckpoint", 1000)
	config.SetDefault("db.journal_size_limit", 67108864)

	// Read tuning parameters from config.
	busyTimeout := config.GetOr[int]("db.busy_timeout", 5000)
	cacheSize := config.GetOr[int]("db.cache_size", -16000)
	mmapSize := config.GetOr[int]("db.mmap_size", 268435456)
	walAutocheckpoint := config.GetOr[int]("db.wal_autocheckpoint", 1000)
	journalSizeLimit := config.GetOr[int]("db.journal_size_limit", 67108864)

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

	// Apply PRAGMAs with configured values.
	pc := pragmaConfig{
		busyTimeout:      busyTimeout,
		cacheSize:        cacheSize,
		mmapSize:         mmapSize,
		walAutocheckpoint: walAutocheckpoint,
		journalSizeLimit: journalSizeLimit,
	}
	if err := applyPragmas(writeDB, readDB, pc); err != nil {
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
		Name: "sqlite",
		OnStop: func(_ context.Context) error {
			// SQLite recommends running PRAGMA optimize before closing.
			// This analyzes tables whose statistics are stale, improving
			// query planner decisions for the next session.
			if global.db != nil {
				_ = global.db.Optimize()
			}
			return Close()
		},
	})

	slog.Info("sqlite: database opened",
		"path", dbPath,
		"read_conns", readConns,
		"busy_timeout_ms", busyTimeout,
		"cache_size", cacheSize,
		"mmap_size", mmapSize,
	)
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

// pragmaConfig holds tuning parameters read from config before applying.
type pragmaConfig struct {
	busyTimeout      int
	cacheSize        int
	mmapSize         int
	walAutocheckpoint int
	journalSizeLimit int
}

func applyPragmas(write, read *sql.DB, pc pragmaConfig) error {
	// PRAGMAs for both pools. journal_mode and synchronous are framework
	// opinions — always WAL + NORMAL for standalone apps.
	shared := []string{
		fmt.Sprintf("PRAGMA busy_timeout = %d", pc.busyTimeout),
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		fmt.Sprintf("PRAGMA cache_size = %d", pc.cacheSize),
		"PRAGMA foreign_keys = ON",
		"PRAGMA temp_store = MEMORY",
		fmt.Sprintf("PRAGMA mmap_size = %d", pc.mmapSize),
	}

	// PRAGMAs only for the write pool.
	writeOnly := []string{
		fmt.Sprintf("PRAGMA journal_size_limit = %d", pc.journalSizeLimit),
		fmt.Sprintf("PRAGMA wal_autocheckpoint = %d", pc.walAutocheckpoint),
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
