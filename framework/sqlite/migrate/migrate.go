package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"
)

// Migration represents a single parsed migration file.
type Migration struct {
	Version int
	Name    string
	Up      []string
	Down    []string
}

// MigrationStatus reports the state of a migration, combining information
// from the migration file (statement counts, checksum) and the database
// record (applied time, execution duration).
//
// When Orphaned is true, the migration exists in the database but has no
// corresponding file. File-derived fields (HasDown, UpStmtCount,
// DownStmtCount, Checksum) are zero-valued for orphaned entries.
type MigrationStatus struct {
	Version       int        `json:"version"`
	Name          string     `json:"name"`
	Applied       bool       `json:"applied"`
	AppliedAt     *time.Time `json:"applied_at,omitempty"`
	ExecutionMs   int64      `json:"execution_ms,omitempty"`
	HasDown       bool       `json:"has_down"`
	UpStmtCount   int        `json:"up_stmt_count"`
	DownStmtCount int        `json:"down_stmt_count"`
	Checksum      string     `json:"checksum"`
	Dirty         bool       `json:"dirty"`
	Orphaned      bool       `json:"orphaned,omitempty"`
}

// Engine manages schema migrations against a SQLite database.
type Engine struct {
	db         *sql.DB
	migrations []Migration
}

// NewEngine creates a migration engine that operates on the given database
// connection. The connection should be the write pool (single-writer).
func NewEngine(db *sql.DB) *Engine {
	return &Engine{db: db}
}

// Collect reads migration files from one or more fs.FS sources, parses them,
// and merges them into a sorted list. Duplicate versions across sources
// cause an error. Nil sources are skipped.
func (e *Engine) Collect(sources ...fs.FS) error {
	seen := make(map[int]string) // version → filename (for duplicate detection)
	var all []Migration

	for _, src := range sources {
		if src == nil {
			continue
		}

		entries, err := fs.ReadDir(src, ".")
		if err != nil {
			return fmt.Errorf("read migration source: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
				continue
			}

			version, name, err := parseFilename(entry.Name())
			if err != nil {
				return err
			}

			if prev, ok := seen[version]; ok {
				return fmt.Errorf("duplicate migration version %d: %s and %s", version, prev, entry.Name())
			}
			seen[version] = entry.Name()

			f, err := src.Open(entry.Name())
			if err != nil {
				return fmt.Errorf("open migration %s: %w", entry.Name(), err)
			}

			parsed, err := Parse(f)
			f.Close()
			if err != nil {
				return fmt.Errorf("parse migration %s: %w", entry.Name(), err)
			}

			all = append(all, Migration{
				Version: version,
				Name:    name,
				Up:      parsed.Up,
				Down:    parsed.Down,
			})
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Version < all[j].Version
	})

	e.migrations = all
	return nil
}

// Up applies all pending migrations in version order. Returns the count
// of applied migrations. Already-applied migrations with changed checksums
// are logged as warnings.
func (e *Engine) Up(ctx context.Context) (int, error) {
	if err := e.ensureTable(ctx); err != nil {
		return 0, err
	}

	records, err := e.appliedRecords(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, m := range e.migrations {
		if rec, ok := records[m.Version]; ok {
			// Already applied — check for checksum drift.
			if rec.checksum != "" {
				cs := Checksum(m)
				if rec.checksum != cs {
					slog.Warn("migration file changed after apply",
						"version", m.Version,
						"name", m.Name,
						"db_checksum", rec.checksum,
						"file_checksum", cs,
					)
				}
			}
			continue
		}

		start := time.Now()
		if err := e.applyUp(ctx, m); err != nil {
			return count, fmt.Errorf("migration %05d_%s up: %w", m.Version, m.Name, err)
		}
		count++
		slog.Info("migration applied",
			"version", m.Version,
			"name", m.Name,
			"direction", "up",
			"duration", time.Since(start),
		)
	}

	return count, nil
}

// UpTo applies pending migrations up to and including the specified version.
// Returns the count of applied migrations. Already-applied migrations with
// changed checksums are logged as warnings.
func (e *Engine) UpTo(ctx context.Context, version int) (int, error) {
	if err := e.ensureTable(ctx); err != nil {
		return 0, err
	}

	records, err := e.appliedRecords(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, m := range e.migrations {
		if m.Version > version {
			break
		}
		if rec, ok := records[m.Version]; ok {
			if rec.checksum != "" {
				cs := Checksum(m)
				if rec.checksum != cs {
					slog.Warn("migration file changed after apply",
						"version", m.Version,
						"name", m.Name,
						"db_checksum", rec.checksum,
						"file_checksum", cs,
					)
				}
			}
			continue
		}

		start := time.Now()
		if err := e.applyUp(ctx, m); err != nil {
			return count, fmt.Errorf("migration %05d_%s up: %w", m.Version, m.Name, err)
		}
		count++
		slog.Info("migration applied",
			"version", m.Version,
			"name", m.Name,
			"direction", "up",
			"duration", time.Since(start),
		)
	}

	return count, nil
}

// Down rolls back the last count applied migrations in reverse version
// order. Returns the count of rolled-back migrations.
func (e *Engine) Down(ctx context.Context, count int) (int, error) {
	if err := e.ensureTable(ctx); err != nil {
		return 0, err
	}

	records, err := e.appliedRecords(ctx)
	if err != nil {
		return 0, err
	}

	// Build list of applied migrations in reverse version order.
	var toRollback []Migration
	for i := len(e.migrations) - 1; i >= 0; i-- {
		m := e.migrations[i]
		if _, ok := records[m.Version]; ok {
			toRollback = append(toRollback, m)
		}
	}

	rolled := 0
	for _, m := range toRollback {
		if rolled >= count {
			break
		}

		if len(m.Down) == 0 {
			return rolled, fmt.Errorf("migration %05d_%s: no Down statements for rollback", m.Version, m.Name)
		}

		start := time.Now()
		if err := e.applyDown(ctx, m); err != nil {
			return rolled, fmt.Errorf("migration %05d_%s down: %w", m.Version, m.Name, err)
		}
		rolled++
		slog.Info("migration rolled back",
			"version", m.Version,
			"name", m.Name,
			"direction", "down",
			"duration", time.Since(start),
		)
	}

	return rolled, nil
}

// DownTo rolls back applied migrations down to but not including the
// specified version. The target version remains applied. Use version 0
// to roll back all migrations.
func (e *Engine) DownTo(ctx context.Context, version int) (int, error) {
	if err := e.ensureTable(ctx); err != nil {
		return 0, err
	}

	records, err := e.appliedRecords(ctx)
	if err != nil {
		return 0, err
	}

	// Build list of applied migrations above the target in reverse order.
	var toRollback []Migration
	for i := len(e.migrations) - 1; i >= 0; i-- {
		m := e.migrations[i]
		if m.Version <= version {
			break
		}
		if _, ok := records[m.Version]; ok {
			toRollback = append(toRollback, m)
		}
	}

	rolled := 0
	for _, m := range toRollback {
		if len(m.Down) == 0 {
			return rolled, fmt.Errorf("migration %05d_%s: no Down statements for rollback", m.Version, m.Name)
		}

		start := time.Now()
		if err := e.applyDown(ctx, m); err != nil {
			return rolled, fmt.Errorf("migration %05d_%s down: %w", m.Version, m.Name, err)
		}
		rolled++
		slog.Info("migration rolled back",
			"version", m.Version,
			"name", m.Name,
			"direction", "down",
			"duration", time.Since(start),
		)
	}

	return rolled, nil
}

// Redo rolls back the last applied migration and re-applies it. Useful
// during development when iterating on a migration file.
func (e *Engine) Redo(ctx context.Context) error {
	if err := e.ensureTable(ctx); err != nil {
		return err
	}

	ver, err := e.Version(ctx)
	if err != nil {
		return err
	}
	if ver == 0 {
		return fmt.Errorf("no applied migrations to redo")
	}

	var target *Migration
	for i := range e.migrations {
		if e.migrations[i].Version == ver {
			target = &e.migrations[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("migration version %d not found in collected migrations", ver)
	}
	if len(target.Down) == 0 {
		return fmt.Errorf("migration %05d_%s: no Down statements for redo", target.Version, target.Name)
	}

	start := time.Now()
	if err := e.applyDown(ctx, *target); err != nil {
		return fmt.Errorf("migration %05d_%s redo down: %w", target.Version, target.Name, err)
	}
	slog.Info("migration rolled back (redo)",
		"version", target.Version,
		"name", target.Name,
		"duration", time.Since(start),
	)

	start = time.Now()
	if err := e.applyUp(ctx, *target); err != nil {
		return fmt.Errorf("migration %05d_%s redo up: %w", target.Version, target.Name, err)
	}
	slog.Info("migration re-applied (redo)",
		"version", target.Version,
		"name", target.Name,
		"duration", time.Since(start),
	)

	return nil
}

// Version returns the highest applied migration version, or 0 if no
// migrations have been applied.
func (e *Engine) Version(ctx context.Context) (int, error) {
	if err := e.ensureTable(ctx); err != nil {
		return 0, err
	}

	var version sql.NullInt64
	err := e.db.QueryRowContext(ctx,
		"SELECT MAX(version) FROM _migrations").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("query current version: %w", err)
	}
	if !version.Valid {
		return 0, nil
	}
	return int(version.Int64), nil
}

// Status returns the status of all known migrations, combining file
// metadata with database records. Includes orphaned entries — migrations
// that exist in the database but have no corresponding file. The result
// is sorted by version. The Dirty field is true when an applied
// migration's checksum differs from its current file content.
func (e *Engine) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := e.ensureTable(ctx); err != nil {
		return nil, err
	}

	records, err := e.appliedRecords(ctx)
	if err != nil {
		return nil, err
	}

	// Track which DB records are matched to files.
	matched := make(map[int]bool, len(e.migrations))

	result := make([]MigrationStatus, 0, len(e.migrations))
	for _, m := range e.migrations {
		cs := Checksum(m)
		s := MigrationStatus{
			Version:       m.Version,
			Name:          m.Name,
			HasDown:       len(m.Down) > 0,
			UpStmtCount:   len(m.Up),
			DownStmtCount: len(m.Down),
			Checksum:      cs,
		}
		if rec, ok := records[m.Version]; ok {
			matched[m.Version] = true
			s.Applied = true
			s.AppliedAt = &rec.appliedAt
			s.ExecutionMs = rec.executionMs
			// Dirty if the DB has a checksum and it differs from the file.
			// Empty DB checksum means the migration was applied before
			// checksum tracking was added — not considered dirty.
			s.Dirty = rec.checksum != "" && rec.checksum != cs
		}
		result = append(result, s)
	}

	// Detect orphaned records — applied in DB but no corresponding file.
	for v, rec := range records {
		if matched[v] {
			continue
		}
		result = append(result, MigrationStatus{
			Version:   v,
			Name:      rec.name,
			Applied:   true,
			AppliedAt: &rec.appliedAt,
			ExecutionMs: rec.executionMs,
			Orphaned:  true,
		})
	}

	// Re-sort by version since orphaned entries may be interleaved.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})

	return result, nil
}

// Pending returns migrations that have not yet been applied, in version
// order. Useful for dry-run reporting or admin dashboard display.
func (e *Engine) Pending(ctx context.Context) ([]Migration, error) {
	if err := e.ensureTable(ctx); err != nil {
		return nil, err
	}

	records, err := e.appliedRecords(ctx)
	if err != nil {
		return nil, err
	}

	var pending []Migration
	for _, m := range e.migrations {
		if _, ok := records[m.Version]; !ok {
			pending = append(pending, m)
		}
	}
	return pending, nil
}

// HasPending returns true if there are unapplied migrations. This is a
// lightweight check — it does not load full migration data.
func (e *Engine) HasPending(ctx context.Context) (bool, error) {
	if err := e.ensureTable(ctx); err != nil {
		return false, err
	}

	records, err := e.appliedRecords(ctx)
	if err != nil {
		return false, err
	}

	for _, m := range e.migrations {
		if _, ok := records[m.Version]; !ok {
			return true, nil
		}
	}
	return false, nil
}

// ValidationResult reports integrity issues between collected migration
// files and the database state. Orphaned entries are applied migrations
// with no corresponding file. Dirty entries are applied migrations whose
// file content has changed since being applied.
type ValidationResult struct {
	Orphaned []OrphanedMigration `json:"orphaned,omitempty"`
	Dirty    []DirtyMigration    `json:"dirty,omitempty"`
}

// Clean returns true when no issues were found.
func (v ValidationResult) Clean() bool {
	return len(v.Orphaned) == 0 && len(v.Dirty) == 0
}

// OrphanedMigration represents a migration that exists in the database
// but has no corresponding file in the collected sources.
type OrphanedMigration struct {
	Version   int       `json:"version"`
	Name      string    `json:"name"`
	AppliedAt time.Time `json:"applied_at"`
}

// DirtyMigration represents a migration whose file content has changed
// since it was applied.
type DirtyMigration struct {
	Version      int    `json:"version"`
	Name         string `json:"name"`
	FileChecksum string `json:"file_checksum"`
	DBChecksum   string `json:"db_checksum"`
}

// Validate checks for integrity issues between collected migration files
// and the database. Returns a ValidationResult with any orphaned records
// (applied but no file) and dirty checksums (file changed after apply).
// Does not modify state — safe to call at any time.
func (e *Engine) Validate(ctx context.Context) (ValidationResult, error) {
	var result ValidationResult

	if err := e.ensureTable(ctx); err != nil {
		return result, err
	}

	records, err := e.appliedRecords(ctx)
	if err != nil {
		return result, err
	}

	// Build set of collected versions for orphan detection.
	collected := make(map[int]bool, len(e.migrations))
	for _, m := range e.migrations {
		collected[m.Version] = true
	}

	// Detect orphaned DB records.
	for v, rec := range records {
		if collected[v] {
			continue
		}
		result.Orphaned = append(result.Orphaned, OrphanedMigration{
			Version:   v,
			Name:      rec.name,
			AppliedAt: rec.appliedAt,
		})
	}
	sort.Slice(result.Orphaned, func(i, j int) bool {
		return result.Orphaned[i].Version < result.Orphaned[j].Version
	})

	// Detect dirty checksums.
	for _, m := range e.migrations {
		rec, ok := records[m.Version]
		if !ok || rec.checksum == "" {
			continue
		}
		cs := Checksum(m)
		if rec.checksum != cs {
			result.Dirty = append(result.Dirty, DirtyMigration{
				Version:      m.Version,
				Name:         m.Name,
				FileChecksum: cs,
				DBChecksum:   rec.checksum,
			})
		}
	}

	return result, nil
}

// Migrations returns the collected migrations for inspection.
func (e *Engine) Migrations() []Migration {
	return e.migrations
}

// Checksum computes a deterministic hash of a migration's parsed Up and
// Down statements. Based on parsed content (not raw file formatting), so
// whitespace-only or comment-only changes don't trigger false positives.
// Returns a 32-character hex string (128-bit truncated SHA-256).
func Checksum(m Migration) string {
	h := sha256.New()
	for i, s := range m.Up {
		if i > 0 {
			h.Write([]byte{'\n'})
		}
		io.WriteString(h, s)
	}
	h.Write([]byte{0})
	for i, s := range m.Down {
		if i > 0 {
			h.Write([]byte{'\n'})
		}
		io.WriteString(h, s)
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (e *Engine) ensureTable(ctx context.Context) error {
	_, err := e.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			version      INTEGER PRIMARY KEY,
			name         TEXT    NOT NULL,
			applied_at   TEXT    NOT NULL DEFAULT (datetime('now')),
			checksum     TEXT    NOT NULL DEFAULT '',
			execution_ms INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("create _migrations table: %w", err)
	}

	// Upgrade existing tables that lack the new columns. ALTER TABLE
	// ADD COLUMN errors on duplicates — we ignore those errors.
	e.db.ExecContext(ctx, "ALTER TABLE _migrations ADD COLUMN checksum TEXT NOT NULL DEFAULT ''")
	e.db.ExecContext(ctx, "ALTER TABLE _migrations ADD COLUMN execution_ms INTEGER NOT NULL DEFAULT 0")

	return nil
}

// appliedRecord holds the database state of a single applied migration.
type appliedRecord struct {
	name        string
	checksum    string
	appliedAt   time.Time
	executionMs int64
}

func (e *Engine) appliedRecords(ctx context.Context) (map[int]appliedRecord, error) {
	rows, err := e.db.QueryContext(ctx,
		"SELECT version, name, applied_at, checksum, execution_ms FROM _migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("query applied records: %w", err)
	}
	defer rows.Close()

	records := make(map[int]appliedRecord)
	for rows.Next() {
		var v int
		var name, at, cs string
		var ms int64
		if err := rows.Scan(&v, &name, &at, &cs, &ms); err != nil {
			return nil, fmt.Errorf("scan applied record: %w", err)
		}
		t, _ := time.Parse("2006-01-02 15:04:05", at)
		records[v] = appliedRecord{
			name:        name,
			checksum:    cs,
			appliedAt:   t,
			executionMs: ms,
		}
	}
	return records, rows.Err()
}

// applyUp executes the migration's Up statements inside a transaction,
// then records the version with its checksum and execution time.
func (e *Engine) applyUp(ctx context.Context, m Migration) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	start := time.Now()
	for i, stmt := range m.Up {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("statement %d/%d: %w\nSQL: %s", i+1, len(m.Up), err, stmt)
		}
	}
	ms := time.Since(start).Milliseconds()

	cs := Checksum(m)
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO _migrations (version, name, checksum, execution_ms) VALUES (?, ?, ?, ?)",
		m.Version, m.Name, cs, ms); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

// applyDown executes the migration's Down statements inside a transaction,
// then removes the version record.
func (e *Engine) applyDown(ctx context.Context, m Migration) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for i, stmt := range m.Down {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("statement %d/%d: %w\nSQL: %s", i+1, len(m.Down), err, stmt)
		}
	}

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM _migrations WHERE version = ?",
		m.Version); err != nil {
		return fmt.Errorf("remove migration record: %w", err)
	}

	return tx.Commit()
}
