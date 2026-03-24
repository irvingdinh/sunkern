package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"sunkern.local/framework/config"
	"sunkern.local/framework/container"
)

type state struct {
	mu     sync.Mutex
	writer *dailyFileWriter
}

var global state

// Load creates the dual-output structured logger and sets it as the slog
// default, after [config.Load] has run. It reads "log.level" from config for
// the handler level (default "INFO"). Both console and file produce JSON with
// identical structure; console output is pretty-printed (2-space indent) for
// readability, file output is compact JSONL for machine consumption. On
// failure it panics (same spirit as config.Load). Call Close to flush and
// close the file writer during shutdown.
func Load() {
	levelStr := config.GetOr[string]("log.level", "INFO")
	var consoleLevel slog.LevelVar
	if err := parseLevel(&consoleLevel, levelStr); err != nil {
		panic(fmt.Sprintf("log: %v", err))
	}

	consoleHandler := slog.NewJSONHandler(&prettyWriter{out: os.Stdout}, &slog.HandlerOptions{
		Level:     &consoleLevel,
		AddSource: true,
	})

	dataDir := config.Get[string]("data_dir")
	logsDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		panic(fmt.Sprintf("log: creating logs directory: %v", err))
	}

	global.mu.Lock()
	global.writer = newDailyFileWriter(logsDir)
	global.mu.Unlock()

	fileHandler := slog.NewJSONHandler(global.writer, &slog.HandlerOptions{
		Level:     &consoleLevel,
		AddSource: true,
	})

	merged := newMergedHandler(consoleHandler, fileHandler)
	root := newContextHandler(merged)
	slog.SetDefault(slog.New(root))

	container.Supply[*slog.LevelVar](&consoleLevel)

	container.AppendHook(container.Hook{
		OnStop: func(_ context.Context) error {
			return Close()
		},
	})
}

// Close flushes and closes the file writer. Safe to call multiple times.
// Normally called via the container shutdown hook; exported for early-abort
// cleanup in the boot sequence.
func Close() error {
	global.mu.Lock()
	defer global.mu.Unlock()
	if global.writer != nil {
		return global.writer.Close()
	}
	return nil
}

// Reset closes the file writer and sets the slog default to a discard
// handler. Intended for tests to get clean state between test cases.
func Reset() {
	global.mu.Lock()
	if global.writer != nil {
		_ = global.writer.Close()
		global.writer = nil
	}
	global.mu.Unlock()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

// parseLevel sets lv from a string like "DEBUG", "INFO", "WARN", "ERROR".
func parseLevel(lv *slog.LevelVar, s string) error {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		lv.Set(slog.LevelDebug)
	case "INFO":
		lv.Set(slog.LevelInfo)
	case "WARN", "WARNING":
		lv.Set(slog.LevelWarn)
	case "ERROR":
		lv.Set(slog.LevelError)
	default:
		return fmt.Errorf("unknown level %q", s)
	}
	return nil
}
