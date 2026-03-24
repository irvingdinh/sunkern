package sqlite

import (
	"log/slog"
	"os"
	"time"
)

// maintenance runs periodic database maintenance tasks in a background
// goroutine: PRAGMA optimize (refreshes query planner statistics) and
// WAL checkpoint when the WAL file exceeds a configurable size threshold.
type maintenance struct {
	db       *DB
	interval time.Duration
	walLimit int64
	done     chan struct{}
}

func newMaintenance(db *DB, interval time.Duration, walLimit int64) *maintenance {
	return &maintenance{
		db:       db,
		interval: interval,
		walLimit: walLimit,
		done:     make(chan struct{}),
	}
}

// start launches the background maintenance loop. The goroutine runs until
// stop is called.
func (m *maintenance) start() {
	go m.loop()
}

// stop signals the background goroutine to exit.
func (m *maintenance) stop() {
	close(m.done)
}

func (m *maintenance) loop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.tick()
		}
	}
}

func (m *maintenance) tick() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("sqlite: maintenance panic recovered", "error", r)
		}
	}()

	// PRAGMA optimize — analyzes tables with stale query planner statistics.
	if err := m.db.Optimize(); err != nil {
		slog.Warn("sqlite: periodic optimize failed", "error", err)
	} else {
		slog.Debug("sqlite: periodic optimize completed")
	}

	// Check WAL file size and force a TRUNCATE checkpoint if it exceeds the
	// configured threshold. The built-in wal_autocheckpoint PRAGMA runs
	// PASSIVE checkpoints (which don't reclaim disk space), so this acts as
	// a backstop for WAL growth under sustained write load.
	if m.walLimit > 0 {
		fi, err := os.Stat(m.db.path + "-wal")
		if err != nil || fi.Size() <= m.walLimit {
			return
		}
		walSize := fi.Size()
		result, err := m.db.Checkpoint()
		if err != nil {
			slog.Warn("sqlite: periodic checkpoint failed",
				"wal_size", walSize,
				"error", err,
			)
		} else {
			slog.Info("sqlite: periodic checkpoint completed",
				"wal_size", walSize,
				"wal_pages", result.WALPages,
				"checkpointed", result.Checkpointed,
			)
		}
	}
}
