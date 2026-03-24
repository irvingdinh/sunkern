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


// ConsoleLevel wraps [slog.LevelVar] for the console (stdout) output sink.
// Resolve it from the container to inspect or change the console log level at
// runtime without restarting the application.
type ConsoleLevel struct{ slog.LevelVar }

// FileLevel wraps [slog.LevelVar] for the file (JSONL) output sink. Resolve
// it from the container to inspect or change the file log level at runtime
// without restarting the application.
type FileLevel struct{ slog.LevelVar }

type state struct {
	mu     sync.Mutex
	writer *dailyFileWriter
}

var global state

// Load creates the dual-output structured logger and sets it as the slog
// default. It must be called after [config.Load].
//
// Two independent log levels control output verbosity:
//
//   - "log.level" (env LOG_LEVEL) — minimum level for the file sink
//     (default "INFO"). File output is compact JSONL for machine consumption.
//   - "log.console.level" (env LOG_CONSOLE_LEVEL) — minimum level for the
//     console sink. Defaults to the value of "log.level" when not set
//     explicitly. Console output is pretty-printed JSON for readability.
//
// Both sinks include source location and produce identical JSON structure;
// only formatting and level filtering differ. On invalid config the function
// panics (same spirit as [config.Load]).
//
// The two levels are supplied to the container as [*ConsoleLevel] and
// [*FileLevel] for runtime adjustment. A shutdown hook is registered to
// flush and close the file writer during graceful shutdown.
func Load() {
	config.SetDefaults(map[string]any{
		"log.level":         "INFO",
		"log.console.level": "",
		"log.sample.debug":  0,
		"log.sample.info":   0,
	})

	fileLevelStr := config.GetOr[string]("log.level", "INFO")
	var fileLevel FileLevel
	if err := parseLevel(&fileLevel.LevelVar, fileLevelStr); err != nil {
		panic(fmt.Sprintf("log: %v", err))
	}

	consoleLevelStr := config.GetOr[string]("log.console.level", "")
	if consoleLevelStr == "" {
		consoleLevelStr = fileLevelStr // inherit from log.level when not set
	}
	var consoleLevel ConsoleLevel
	if err := parseLevel(&consoleLevel.LevelVar, consoleLevelStr); err != nil {
		panic(fmt.Sprintf("log: %v", err))
	}

	consoleHandler := slog.NewJSONHandler(&prettyWriter{out: os.Stdout}, &slog.HandlerOptions{
		Level:     &consoleLevel.LevelVar,
		AddSource: true,
	})

	logsDir := filepath.Join(config.DataDir(), "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		panic(fmt.Sprintf("log: creating logs directory: %v", err))
	}

	global.mu.Lock()
	global.writer = newDailyFileWriter(logsDir)
	global.mu.Unlock()

	fileHandler := slog.NewJSONHandler(global.writer, &slog.HandlerOptions{
		Level:     &fileLevel.LevelVar,
		AddSource: true,
	})

	merged := newMergedHandler(consoleHandler, fileHandler)

	// Apply per-level sampling when configured. Sampling drops a fraction
	// of DEBUG/INFO records before they reach either sink, reducing log
	// volume on high-traffic paths without losing WARN/ERROR visibility.
	rate := SamplingRate{
		Debug: config.GetOr[int]("log.sample.debug", 0),
		Info:  config.GetOr[int]("log.sample.info", 0),
	}
	sampled := newSamplingHandler(merged, rate)

	root := newContextHandler(sampled)
	slog.SetDefault(slog.New(root))

	container.Supply[*ConsoleLevel](&consoleLevel)
	container.Supply[*FileLevel](&fileLevel)

	container.AppendHook(container.Hook{
		Name: "log",
		OnStop: func(_ context.Context) error {
			return Close()
		},
	})
}

// Flush writes any buffered log data to the underlying file. Use this before
// querying recent log entries to ensure they are visible on disk. Safe to call
// concurrently; returns nil when no writer is active.
func Flush() error {
	global.mu.Lock()
	defer global.mu.Unlock()
	if global.writer != nil {
		return global.writer.Flush()
	}
	return nil
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
