package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
)

// Stats holds diagnostic information about the SQLite database. All fields
// are JSON-serializable — designed for the admin dashboard.
type Stats struct {
	// Path is the database file path.
	Path string `json:"path"`
	// FileSize is the main database file size in bytes.
	FileSize int64 `json:"file_size"`
	// WALSize is the WAL file size in bytes. Zero after a truncating
	// checkpoint or when WAL mode is not active.
	WALSize int64 `json:"wal_size"`
	// PageSize is the SQLite page size in bytes (typically 4096).
	PageSize int64 `json:"page_size"`
	// PageCount is the total number of pages in the database.
	PageCount int64 `json:"page_count"`
	// FreelistCount is the number of unused pages available for reuse.
	FreelistCount int64 `json:"freelist_count"`
	// Tables is the count of user tables (excludes sqlite_ internal tables
	// and the _migrations table).
	Tables int `json:"tables"`
	// Indexes is the count of user-created indexes.
	Indexes int `json:"indexes"`
	// WritePool reports the write connection pool status.
	WritePool PoolStats `json:"write_pool"`
	// ReadPool reports the read connection pool status.
	ReadPool PoolStats `json:"read_pool"`
}

// PoolStats summarizes the state of a database/sql connection pool.
type PoolStats struct {
	MaxOpen      int           `json:"max_open"`
	Open         int           `json:"open"`
	InUse        int           `json:"in_use"`
	Idle         int           `json:"idle"`
	WaitCount    int64         `json:"wait_count"`
	WaitDuration time.Duration `json:"wait_duration"`
}

// Stats collects diagnostic information about the database. Safe to call
// concurrently — uses the read pool for PRAGMA queries to avoid blocking
// writes.
func (db *DB) Stats() (Stats, error) {
	s := Stats{
		Path:      db.path,
		WritePool: poolStatsFrom(db.write.Stats()),
		ReadPool:  poolStatsFrom(db.read.Stats()),
	}

	// File sizes — errors are non-fatal (file might be briefly locked).
	if fi, err := os.Stat(db.path); err == nil {
		s.FileSize = fi.Size()
	}
	if fi, err := os.Stat(db.path + "-wal"); err == nil {
		s.WALSize = fi.Size()
	}

	// PRAGMA queries on the read pool.
	pragmas := []struct {
		query string
		dest  *int64
	}{
		{"PRAGMA page_size", &s.PageSize},
		{"PRAGMA page_count", &s.PageCount},
		{"PRAGMA freelist_count", &s.FreelistCount},
	}
	for _, p := range pragmas {
		if err := db.read.QueryRow(p.query).Scan(p.dest); err != nil {
			return s, fmt.Errorf("sqlite stats: %s: %w", p.query, err)
		}
	}

	// Table and index counts (exclude internal tables and _migrations).
	if err := db.read.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name != '_migrations'",
	).Scan(&s.Tables); err != nil {
		return s, fmt.Errorf("sqlite stats: count tables: %w", err)
	}
	if err := db.read.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name NOT LIKE 'sqlite_%'",
	).Scan(&s.Indexes); err != nil {
		return s, fmt.Errorf("sqlite stats: count indexes: %w", err)
	}

	return s, nil
}

// CheckpointResult holds the outcome of a WAL checkpoint.
type CheckpointResult struct {
	// WALPages is the number of frames in the WAL before the checkpoint.
	WALPages int
	// Checkpointed is the number of frames successfully moved to the main
	// database file.
	Checkpointed int
}

// Checkpoint forces a WAL checkpoint using TRUNCATE mode — all WAL frames
// are moved into the main database file and the WAL is truncated to zero
// bytes. Returns the frame counts before and after. Must run on the write
// pool (serialized with writes).
func (db *DB) Checkpoint() (CheckpointResult, error) {
	var busy, log, checkpointed int
	err := db.write.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &log, &checkpointed)
	if err != nil {
		return CheckpointResult{}, fmt.Errorf("sqlite checkpoint: %w", err)
	}
	if busy != 0 {
		return CheckpointResult{WALPages: log, Checkpointed: checkpointed},
			fmt.Errorf("sqlite checkpoint: database was busy, only %d of %d frames checkpointed", checkpointed, log)
	}
	return CheckpointResult{WALPages: log, Checkpointed: checkpointed}, nil
}

// Optimize runs PRAGMA optimize, which analyzes tables whose query planner
// statistics are stale. SQLite recommends calling this periodically (e.g.,
// daily cron) and on graceful shutdown. The framework calls it automatically
// during shutdown.
func (db *DB) Optimize() error {
	_, err := db.write.Exec("PRAGMA optimize")
	if err != nil {
		return fmt.Errorf("sqlite optimize: %w", err)
	}
	return nil
}

// IntegrityCheck runs PRAGMA quick_check on the database. Returns nil if
// the database is intact, or an error listing all corruption issues found.
// Uses the read pool — safe to call while the application is running.
func (db *DB) IntegrityCheck() error {
	rows, err := db.read.Query("PRAGMA quick_check")
	if err != nil {
		return fmt.Errorf("sqlite integrity check: %w", err)
	}
	defer rows.Close()

	var issues []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("sqlite integrity check: scan: %w", err)
		}
		if result != "ok" {
			issues = append(issues, result)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sqlite integrity check: %w", err)
	}
	if len(issues) > 0 {
		return fmt.Errorf("sqlite integrity check failed: %s", strings.Join(issues, "; "))
	}
	return nil
}

// Backup creates a standalone copy of the database at destPath using VACUUM
// INTO. The copy is fully defragmented and self-contained — suitable for
// archival, transfer, or disaster recovery. The destination file must not
// already exist. Runs on the write pool to ensure a consistent snapshot.
func (db *DB) Backup(destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("sqlite backup: destination already exists: %s", destPath)
	}
	if _, err := db.write.Exec("VACUUM INTO ?", destPath); err != nil {
		// Clean up partial file on failure.
		os.Remove(destPath)
		return fmt.Errorf("sqlite backup: %w", err)
	}
	return nil
}

// Health pings both the read and write connection pools. Returns nil if the
// database is reachable, or an error describing which pool failed. Suitable
// for /health endpoints — fast and non-blocking.
func (db *DB) Health(ctx context.Context) error {
	if err := db.write.PingContext(ctx); err != nil {
		return fmt.Errorf("sqlite health: write pool: %w", err)
	}
	if err := db.read.PingContext(ctx); err != nil {
		return fmt.Errorf("sqlite health: read pool: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func poolStatsFrom(s sql.DBStats) PoolStats {
	return PoolStats{
		MaxOpen:      s.MaxOpenConnections,
		Open:         s.OpenConnections,
		InUse:        s.InUse,
		Idle:         s.Idle,
		WaitCount:    s.WaitCount,
		WaitDuration: s.WaitDuration,
	}
}
