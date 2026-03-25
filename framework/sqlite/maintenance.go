package sqlite

import (
	"log/slog"
	"os"
	"time"
)

// maintenance runs periodic database maintenance tasks in a background
// goroutine: PRAGMA optimize (refreshes query planner statistics) and
// WAL checkpoint when the WAL file exceeds a configurable size threshold.
//
// Checkpoints can also be triggered proactively via the WAL hook — when a
// transaction commits and the WAL frame count exceeds the page threshold,
// a signal is sent to the maintenance goroutine to checkpoint immediately
// instead of waiting for the next tick.
type maintenance struct {
	db       *DB
	interval time.Duration
	walLimit int64
	// walPageLimit is the WAL frame count threshold for proactive
	// checkpoints. Derived from walLimit / page_size at construction.
	walPageLimit int
	// walSignal receives a non-blocking signal from the WAL hook when the
	// frame count exceeds walPageLimit. Buffered with capacity 1 to
	// coalesce rapid commits.
	walSignal chan struct{}
	done      chan struct{}
}

func newMaintenance(db *DB, interval time.Duration, walLimit int64, walPageLimit int) *maintenance {
	return &maintenance{
		db:           db,
		interval:     interval,
		walLimit:     walLimit,
		walPageLimit: walPageLimit,
		walSignal:    make(chan struct{}, 1),
		done:         make(chan struct{}),
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

// notifyWAL sends a non-blocking signal to the maintenance goroutine that
// the WAL has grown past the page threshold. Called from the WAL hook in
// the committing goroutine — must not block.
func (m *maintenance) notifyWAL() {
	select {
	case m.walSignal <- struct{}{}:
	default:
		// Already signaled, coalesce.
	}
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
		case <-m.walSignal:
			m.checkpoint()
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
	m.checkpoint()
}

// checkpoint checks the WAL file size and runs a TRUNCATE checkpoint if it
// exceeds the configured threshold. Called from both the periodic tick and
// the proactive WAL hook signal.
func (m *maintenance) checkpoint() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("sqlite: checkpoint panic recovered", "error", r)
		}
	}()

	if m.walLimit <= 0 {
		return
	}

	fi, err := os.Stat(m.db.path + "-wal")
	if err != nil || fi.Size() <= m.walLimit {
		return
	}
	walSize := fi.Size()
	result, err := m.db.Checkpoint()
	if err != nil {
		slog.Warn("sqlite: checkpoint failed",
			"wal_size", walSize,
			"trigger", "proactive",
			"error", err,
		)
	} else {
		slog.Info("sqlite: checkpoint completed",
			"wal_size", walSize,
			"trigger", "proactive",
			"wal_pages", result.WALPages,
			"checkpointed", result.Checkpointed,
		)
	}
}
