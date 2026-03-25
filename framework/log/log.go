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
// default. It must be called after [config.Load].
//
// Configuration keys:
//
//   - "log.level" (env LOG_LEVEL, default "INFO") — minimum level for both
//     console and file sinks.
//   - "log.format" (env LOG_FORMAT, default "json") — console output format.
//     "json" writes compact single-line JSON (production-friendly).
//     "json-pretty" writes 2-space indented JSON (development-friendly).
//     File output is always compact JSONL regardless of this setting.
//
// Both sinks include source location and produce the same JSON structure; only
// formatting differs. On invalid config the function panics.
//
// Application boot calls Load once. Tests may call Load again after reloading
// config to replace the package-local logger state, matching the model used by
// [config.Load]. The shutdown hook is registered once per container and closes
// the file writer during graceful shutdown.
func Load() {
	config.SetDefault("log.level", "INFO")
	config.SetDefault("log.format", "json")

	levelStr := config.GetOr[string]("log.level", "INFO")
	var level slog.LevelVar
	if err := parseLevel(&level, levelStr); err != nil {
		panic(fmt.Sprintf("log: %v", err))
	}

	consoleOut := consoleWriter(config.GetOr[string]("log.format", "json"))
	consoleHandler := slog.NewJSONHandler(consoleOut, &slog.HandlerOptions{
		Level:     &level,
		AddSource: true,
	})

	logsDir := filepath.Join(config.DataDir(), "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		panic(fmt.Sprintf("log: creating logs directory: %v", err))
	}

	writer := newDailyFileWriter(logsDir)
	if prev := swapWriter(writer); prev != nil {
		_ = prev.Close()
	}

	fileHandler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:     &level,
		AddSource: true,
	})

	root := newContextHandler(newMergedHandler(consoleHandler, fileHandler))
	slog.SetDefault(slog.New(root))

	ensureHook()
}

// Flush writes any buffered log data to the underlying file. Safe to call
// concurrently; returns nil when no writer is active.
func Flush() error {
	global.mu.Lock()
	writer := global.writer
	global.mu.Unlock()
	if writer != nil {
		return writer.Flush()
	}
	return nil
}

// Close flushes and closes the file writer. Safe to call multiple times.
// Normally called via the container shutdown hook; exported for early-abort
// cleanup in the boot sequence.
func Close() error {
	global.mu.Lock()
	writer := global.writer
	global.writer = nil
	global.mu.Unlock()
	if writer != nil {
		return writer.Close()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func swapWriter(next *dailyFileWriter) *dailyFileWriter {
	global.mu.Lock()
	prev := global.writer
	global.writer = next
	global.mu.Unlock()
	return prev
}

func ensureHook() {
	for _, h := range container.Global().Hooks() {
		if h.Name == "log" {
			return
		}
	}
	container.AppendHook(container.Hook{
		Name: "log",
		OnStop: func(_ context.Context) error {
			return Close()
		},
	})
}

// consoleWriter returns the io.Writer for the console sink based on the
// configured format. Panics on unrecognized values.
func consoleWriter(format string) io.Writer {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return os.Stdout
	case "json-pretty":
		return &prettyWriter{out: os.Stdout}
	default:
		panic(fmt.Sprintf("log: unknown format %q (expected \"json\" or \"json-pretty\")", format))
	}
}

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
