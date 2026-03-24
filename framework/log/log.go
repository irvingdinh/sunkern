package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"sunkern.local/framework/config"
	"sunkern.local/framework/container"
)

// writer is the package-level file writer, set by Init and closed by Close.
var writer *dailyFileWriter

// Init creates the dual-output structured logger and sets it as the slog
// default. It reads "log.level" from config for the console handler level
// (default "INFO") and writes file logs to {data_dir}/logs/. Call Close to
// flush and close the file writer during shutdown.
func Init() error {
	levelStr := config.GetOr[string]("log.level", "INFO")
	var consoleLevel slog.LevelVar
	if err := parseLevel(&consoleLevel, levelStr); err != nil {
		return err
	}

	consoleHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: &consoleLevel,
	})

	dataDir := config.Get[string]("data_dir")
	logsDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fmt.Errorf("log: creating logs directory: %w", err)
	}

	writer = newDailyFileWriter(logsDir)

	fileHandler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	})

	dual := newDualHandler(consoleHandler, fileHandler)
	root := newContextHandler(dual)
	slog.SetDefault(slog.New(root))

	container.Supply[*slog.LevelVar](&consoleLevel)

	container.AppendHook(container.Hook{
		OnStop: func(_ context.Context) error {
			return Close()
		},
	})

	return nil
}

// Close flushes and closes the file writer. Safe to call multiple times.
// Normally called via the container shutdown hook; exported for early-abort
// cleanup in the boot sequence.
func Close() error {
	if writer != nil {
		return writer.Close()
	}
	return nil
}

// Reset closes the file writer and sets the slog default to a discard
// handler. Intended for tests to get clean state between test cases.
func Reset() {
	_ = Close()
	writer = nil
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
		return fmt.Errorf("log: unknown level %q", s)
	}
	return nil
}
