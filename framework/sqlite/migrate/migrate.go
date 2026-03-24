package migrate

import (
	"context"
	"database/sql"
	"fmt"
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

// MigrationStatus reports the state of a migration.
type MigrationStatus struct {
	Version   int
	Name      string
	Applied   bool
	AppliedAt *time.Time
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
// of applied migrations.
func (e *Engine) Up(ctx context.Context) (int, error) {
	if err := e.ensureTable(ctx); err != nil {
		return 0, err
	}

	applied, err := e.appliedVersions(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, m := range e.migrations {
		if applied[m.Version] {
			continue
		}

		if err := e.applyMigration(ctx, m, DirectionUp); err != nil {
			return count, fmt.Errorf("migration %05d_%s up: %w", m.Version, m.Name, err)
		}
		count++
		slog.Info("migration applied",
			"version", m.Version,
			"name", m.Name,
			"direction", "up",
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

	applied, err := e.appliedVersions(ctx)
	if err != nil {
		return 0, err
	}

	// Build list of applied migrations in reverse version order.
	var toRollback []Migration
	for i := len(e.migrations) - 1; i >= 0; i-- {
		m := e.migrations[i]
		if applied[m.Version] {
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

		if err := e.applyMigration(ctx, m, DirectionDown); err != nil {
			return rolled, fmt.Errorf("migration %05d_%s down: %w", m.Version, m.Name, err)
		}
		rolled++
		slog.Info("migration rolled back",
			"version", m.Version,
			"name", m.Name,
			"direction", "down",
		)
	}

	return rolled, nil
}

// Status returns the status of all known migrations.
func (e *Engine) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := e.ensureTable(ctx); err != nil {
		return nil, err
	}

	rows, err := e.db.QueryContext(ctx,
		"SELECT version, applied_at FROM _migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("query _migrations: %w", err)
	}
	defer rows.Close()

	appliedAt := make(map[int]time.Time)
	for rows.Next() {
		var version int
		var at string
		if err := rows.Scan(&version, &at); err != nil {
			return nil, fmt.Errorf("scan _migrations: %w", err)
		}
		t, _ := time.Parse("2006-01-02 15:04:05", at)
		appliedAt[version] = t
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate _migrations: %w", err)
	}

	result := make([]MigrationStatus, len(e.migrations))
	for i, m := range e.migrations {
		s := MigrationStatus{
			Version: m.Version,
			Name:    m.Name,
		}
		if t, ok := appliedAt[m.Version]; ok {
			s.Applied = true
			s.AppliedAt = &t
		}
		result[i] = s
	}

	return result, nil
}

// Migrations returns the collected migrations for inspection.
func (e *Engine) Migrations() []Migration {
	return e.migrations
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (e *Engine) ensureTable(ctx context.Context) error {
	_, err := e.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			version    INTEGER PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return fmt.Errorf("create _migrations table: %w", err)
	}
	return nil
}

func (e *Engine) appliedVersions(ctx context.Context) (map[int]bool, error) {
	rows, err := e.db.QueryContext(ctx, "SELECT version FROM _migrations")
	if err != nil {
		return nil, fmt.Errorf("query applied versions: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan applied version: %w", err)
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func (e *Engine) applyMigration(ctx context.Context, m Migration, dir Direction) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmts := m.Up
	if dir == DirectionDown {
		stmts = m.Down
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec: %w\nstatement: %s", err, stmt)
		}
	}

	switch dir {
	case DirectionUp:
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO _migrations (version, name) VALUES (?, ?)",
			m.Version, m.Name); err != nil {
			return fmt.Errorf("record migration: %w", err)
		}
	case DirectionDown:
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM _migrations WHERE version = ?",
			m.Version); err != nil {
			return fmt.Errorf("remove migration record: %w", err)
		}
	}

	return tx.Commit()
}
