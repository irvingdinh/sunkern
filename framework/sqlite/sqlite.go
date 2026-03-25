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
	"time"

	"sunkern.local/framework/config"
	"sunkern.local/framework/container"
	"sunkern.local/framework/sqlite/driver"
)

// DB holds the dual read/write connection pools to the SQLite database.
// The write pool has MaxOpenConns=1 to serialize all writes and eliminate
// SQLITE_BUSY errors. The read pool has MaxOpenConns matching GOMAXPROCS
// for concurrent reads.
//
// Both pools use driver.Connector, which applies initialization PRAGMAs to
// every new connection. This ensures all pooled connections have consistent
// settings regardless of when they are created by database/sql.
type DB struct {
	write     *sql.DB
	read      *sql.DB
	path      string
	writeConn *driver.Connector
	readConn  *driver.Connector
	maint     *maintenance
}

// WriteDB returns the write-only connection pool (MaxOpenConns=1).
func (db *DB) WriteDB() *sql.DB { return db.write }

// ReadDB returns the read-only connection pool.
func (db *DB) ReadDB() *sql.DB { return db.read }

// Path returns the database file path.
func (db *DB) Path() string { return db.path }

// SetUpdateHook registers a row-change notification callback on the write
// pool. The callback fires for every INSERT, UPDATE, and DELETE. Pass nil
// to disable.
//
// The hook is set on the write connector and takes effect on the next
// connection created by the pool. To apply immediately, idle connections
// are recycled so the pool creates a fresh connection with the hook
// installed.
func (db *DB) SetUpdateHook(fn driver.UpdateFunc) {
	db.writeConn.SetUpdateHook(fn)
	// Force the write pool to drop idle connections. The next query
	// gets a new connection from the connector (which now has the hook).
	// Write pool has MaxOpenConns=1, so this recycles the single idle
	// connection.
	db.write.SetMaxIdleConns(0)
	db.write.SetMaxIdleConns(1)
}

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
	config.SetDefault("db.optimize_interval", "1h")
	config.SetDefault("db.wal_checkpoint_threshold", 104857600) // 100 MB
	config.SetDefault("db.trace", false)

	config.Describe("db.busy_timeout", "Milliseconds to wait for locks before returning SQLITE_BUSY")
	config.Describe("db.cache_size", "Page cache size (negative = KB, e.g. -16000 = 16MB)")
	config.Describe("db.mmap_size", "Memory-mapped I/O limit in bytes (0 to disable)")
	config.Describe("db.wal_autocheckpoint", "Auto-checkpoint threshold in WAL pages")
	config.Describe("db.journal_size_limit", "Max WAL file size after checkpoint in bytes")
	config.Describe("db.optimize_interval", "Interval between PRAGMA optimize runs")
	config.Describe("db.wal_checkpoint_threshold", "WAL size in bytes that triggers proactive checkpoint")
	config.Describe("db.trace", "Enable SQL tracing to slog.Debug (expanded SQL + timing)")

	// Read tuning parameters from config.
	busyTimeout := config.GetOr[int]("db.busy_timeout", 5000)
	cacheSize := config.GetOr[int]("db.cache_size", -16000)
	mmapSize := config.GetOr[int]("db.mmap_size", 268435456)
	walAutocheckpoint := config.GetOr[int]("db.wal_autocheckpoint", 1000)
	journalSizeLimit := config.GetOr[int]("db.journal_size_limit", 67108864)

	// Build connectors that apply PRAGMAs to every new connection.
	// Shared PRAGMAs are applied to both pools; write-only PRAGMAs only
	// to the write pool.
	writeConn := driver.NewConnector("file:" + dbPath)
	readConn := driver.NewConnector("file:" + dbPath + "?mode=ro")

	// Shared PRAGMAs — applied to every connection in both pools.
	shared := map[string]any{
		"busy_timeout": busyTimeout,
		"journal_mode": "WAL",
		"synchronous":  "NORMAL",
		"cache_size":   cacheSize,
		"foreign_keys": "ON",
		"temp_store":   "MEMORY",
		"mmap_size":    mmapSize,
	}
	for k, v := range shared {
		writeConn.SetPragma(k, v)
		readConn.SetPragma(k, v)
	}

	// Write-only PRAGMAs.
	writeConn.SetPragma("journal_size_limit", journalSizeLimit)
	writeConn.SetPragma("wal_autocheckpoint", walAutocheckpoint)

	// SQL tracing — opt-in via DB_TRACE=true. Logs every statement to
	// slog.Debug with expanded SQL and execution time. Useful for
	// debugging but adds overhead; leave disabled in production.
	if config.GetOr[bool]("db.trace", false) {
		traceFn := func(info driver.TraceInfo) {
			switch info.EventType {
			case driver.TraceStmt:
				slog.Debug("sqlite: trace", "event", "stmt", "sql", info.SQL)
			case driver.TraceProfile:
				slog.Debug("sqlite: trace", "event", "profile",
					"sql", info.SQL,
					"duration_us", info.Duration.Microseconds(),
				)
			}
		}
		mask := driver.TraceStmt | driver.TraceProfile
		writeConn.SetTrace(traceFn, mask)
		readConn.SetTrace(traceFn, mask)
	}

	writeDB := sql.OpenDB(writeConn)
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)

	readConns := runtime.GOMAXPROCS(0)
	if readConns < 4 {
		readConns = 4
	}
	readDB := sql.OpenDB(readConn)
	readDB.SetMaxOpenConns(readConns)
	readDB.SetMaxIdleConns(readConns)

	// Verify connectivity (also forces the first connection through the
	// connector, which applies PRAGMAs).
	if err := writeDB.Ping(); err != nil {
		writeDB.Close()
		readDB.Close()
		panic(fmt.Sprintf("sqlite: ping write pool: %v", err))
	}
	if err := readDB.Ping(); err != nil {
		writeDB.Close()
		readDB.Close()
		panic(fmt.Sprintf("sqlite: ping read pool: %v", err))
	}

	global.mu.Lock()
	global.db = &DB{
		write:     writeDB,
		read:      readDB,
		path:      dbPath,
		writeConn: writeConn,
		readConn:  readConn,
	}
	global.mu.Unlock()

	// Background maintenance: periodic PRAGMA optimize + WAL checkpoint
	// when the WAL exceeds a configurable size threshold.
	optimizeInterval := config.GetOr[time.Duration]("db.optimize_interval", time.Hour)
	walCheckpointThreshold := int64(config.GetOr[int]("db.wal_checkpoint_threshold", 104857600))

	// Derive WAL page limit for proactive checkpointing. The WAL hook
	// reports frame count (not bytes), so we convert the byte threshold
	// to pages using the configured page size (default 4096).
	pageSize := config.GetOr[int]("db.page_size", 4096)
	walPageLimit := 0
	if walCheckpointThreshold > 0 && pageSize > 0 {
		walPageLimit = int(walCheckpointThreshold / int64(pageSize))
	}

	var maint *maintenance
	if optimizeInterval > 0 {
		maint = newMaintenance(global.db, optimizeInterval, walCheckpointThreshold, walPageLimit)
		global.db.maint = maint

		// Install WAL hook on the write connector for proactive checkpoint
		// triggering. When a commit pushes the WAL past the page threshold,
		// the hook signals the maintenance goroutine to checkpoint
		// immediately instead of waiting for the next tick.
		if walPageLimit > 0 {
			writeConn.SetWALHook(func(_ string, pages int) {
				if pages >= walPageLimit {
					maint.notifyWAL()
				}
			})
		}
	}

	container.AppendHook(container.Hook{
		Name: "sqlite",
		OnStart: func(_ context.Context) error {
			if maint != nil {
				maint.start()
				slog.Debug("sqlite: maintenance started",
					"optimize_interval", maint.interval,
					"wal_checkpoint_threshold", maint.walLimit,
					"wal_page_limit", maint.walPageLimit,
				)
			}
			return nil
		},
		OnStop: func(_ context.Context) error {
			if maint != nil {
				maint.stop()
			}
			// SQLite recommends running PRAGMA optimize before closing.
			// This analyzes tables whose statistics are stale, improving
			// query planner decisions for the next session.
			if global.db != nil {
				_ = global.db.Optimize()
			}
			return Close()
		},
	})

	logAttrs := []any{
		"path", dbPath,
		"read_conns", readConns,
		"busy_timeout_ms", busyTimeout,
		"cache_size", cacheSize,
		"mmap_size", mmapSize,
		"optimize_interval", optimizeInterval,
	}
	if config.GetOr[bool]("db.trace", false) {
		logAttrs = append(logAttrs, "trace", true)
	}
	slog.Info("sqlite: database opened", logAttrs...)
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

// GetPragma reads the current value of a PRAGMA from the database. Uses the
// read pool. The returned value is the native type from SQLite (int64 for
// integers, string for text).
func (db *DB) GetPragma(key string) (any, error) {
	var val any
	if err := db.read.QueryRow("PRAGMA " + key).Scan(&val); err != nil {
		return nil, fmt.Errorf("sqlite: get pragma %s: %w", key, err)
	}
	return val, nil
}

// SetPragma changes a PRAGMA at runtime on all connections in both pools.
// It also updates both connectors so that future connections inherit the
// change.
//
// Safe PRAGMAs for runtime changes: cache_size, mmap_size, busy_timeout.
// Unsafe (do not change at runtime): journal_mode, synchronous, foreign_keys.
func (db *DB) SetPragma(key string, value any) error {
	pragma := fmt.Sprintf("PRAGMA %s = %v", key, value)

	// Update connectors so new connections inherit the change.
	db.writeConn.SetPragma(key, value)
	db.readConn.SetPragma(key, value)

	// Apply to write pool (1 connection — always hits it).
	if _, err := db.write.Exec(pragma); err != nil {
		return fmt.Errorf("sqlite: set pragma %s: write pool: %w", key, err)
	}

	// Apply to all read pool connections by grabbing each one.
	// Connections that are currently in-use will get the updated PRAGMA
	// the next time they are recycled (via the connector).
	maxConns := db.read.Stats().MaxOpenConnections
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conns := make([]*sql.Conn, 0, maxConns)
	for range maxConns {
		c, err := db.read.Conn(ctx)
		if err != nil {
			break
		}
		_, _ = c.ExecContext(ctx, pragma)
		conns = append(conns, c)
	}
	for _, c := range conns {
		c.Close() // returns to pool
	}

	return nil
}

// Pragmas returns the current values of all framework-managed PRAGMAs.
// Uses the read pool for per-connection PRAGMAs and the write pool for
// write-only PRAGMAs. Designed for admin dashboard introspection.
func (db *DB) Pragmas() map[string]any {
	m := make(map[string]any, 10)

	// Read-pool PRAGMAs (per-connection settings).
	readKeys := []string{
		"journal_mode", "synchronous", "cache_size", "mmap_size",
		"busy_timeout", "foreign_keys", "temp_store",
	}
	for _, key := range readKeys {
		var val any
		if err := db.read.QueryRow("PRAGMA " + key).Scan(&val); err == nil {
			m[key] = val
		}
	}

	// Write-pool PRAGMAs (write-only settings).
	writeKeys := []string{"wal_autocheckpoint", "journal_size_limit"}
	for _, key := range writeKeys {
		var val any
		if err := db.write.QueryRow("PRAGMA " + key).Scan(&val); err == nil {
			m[key] = val
		}
	}

	return m
}
