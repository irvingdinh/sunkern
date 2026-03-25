package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"sunkern.local/framework/config"
	"sunkern.local/framework/container"
)

func setup(t *testing.T) {
	t.Helper()
	t.Setenv("DATA_DIR", t.TempDir())
	container.Reset()
	config.Load()
}

func nonEmptyLines(s string) []string {
	var result []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}

func TestLoadCreatesLogDirectory(t *testing.T) {
	setup(t)

	Load()
	defer Close()

	logsDir := filepath.Join(config.DataDir(), "logs")
	info, err := os.Stat(logsDir)
	if err != nil {
		t.Fatalf("logs directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("logs path is not a directory")
	}
}

func TestLoadSetsDefaultLogger(t *testing.T) {
	setup(t)

	Load()
	defer Close()

	slog.Info("test message")
	if err := Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	logsDir := filepath.Join(config.DataDir(), "logs")
	entries, _ := filepath.Glob(filepath.Join(logsDir, "*.log"))
	if len(entries) == 0 {
		t.Fatal("expected at least one log file after logging")
	}

	data, err := os.ReadFile(entries[0])
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(data), "test message") {
		t.Errorf("log file does not contain expected message; got: %s", string(data))
	}
}

func TestLoadWithCustomLevel(t *testing.T) {
	setup(t)
	t.Setenv("LOG_LEVEL", "ERROR")
	config.Load()

	Load()
	defer Close()

	// ERROR level should suppress INFO messages — verify by checking the
	// log file contains no INFO entry.
	slog.Info("should be filtered")
	slog.Error("should appear")
	if err := Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	logsDir := filepath.Join(config.DataDir(), "logs")
	entries, _ := filepath.Glob(filepath.Join(logsDir, "*.log"))
	if len(entries) == 0 {
		t.Fatal("expected at least one log file")
	}

	data, err := os.ReadFile(entries[0])
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if strings.Contains(string(data), "should be filtered") {
		t.Error("INFO message should have been filtered at ERROR level")
	}
	if !strings.Contains(string(data), "should appear") {
		t.Error("ERROR message should appear in log file")
	}
}

func TestLoadPanicsOnInvalidLogLevel(t *testing.T) {
	setup(t)
	t.Setenv("LOG_LEVEL", "not-a-level")
	config.Load()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid LOG_LEVEL")
		}
	}()
	Load()
}

func TestConsoleFormatJsonIsCompact(t *testing.T) {
	setup(t)

	// Default format is "json" — console should write compact single-line JSON.
	// We test indirectly through the consoleWriter helper.
	w := consoleWriter("json")
	if _, ok := w.(*prettyWriter); ok {
		t.Fatal("json format should not use prettyWriter")
	}
}

func TestConsoleFormatJsonPrettyUsesIndent(t *testing.T) {
	setup(t)

	w := consoleWriter("json-pretty")
	if _, ok := w.(*prettyWriter); !ok {
		t.Fatal("json-pretty format should use prettyWriter")
	}
}

func TestLoadPanicsOnInvalidFormat(t *testing.T) {
	setup(t)
	t.Setenv("LOG_FORMAT", "yaml")
	config.Load()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid LOG_FORMAT")
		}
	}()
	Load()
}

func TestLoadReloadsWithoutReset(t *testing.T) {
	setup(t)

	Load()

	t.Setenv("LOG_LEVEL", "ERROR")
	config.Load()
	Load()
	defer Close()

	// After reload with ERROR level, INFO should be filtered.
	slog.Info("filtered after reload")
	slog.Error("visible after reload")
	if err := Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	logsDir := filepath.Join(config.DataDir(), "logs")
	entries, _ := filepath.Glob(filepath.Join(logsDir, "*.log"))
	if len(entries) == 0 {
		t.Fatal("expected at least one log file")
	}

	data, err := os.ReadFile(entries[0])
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if strings.Contains(string(data), "filtered after reload") {
		t.Error("INFO message should have been filtered after reload to ERROR level")
	}
	if !strings.Contains(string(data), "visible after reload") {
		t.Error("ERROR message should appear after reload")
	}

	logHooks := 0
	for _, h := range container.Global().Hooks() {
		if h.Name == "log" {
			logHooks++
		}
	}
	if logHooks != 1 {
		t.Fatalf("log hook count = %d, want 1", logHooks)
	}
}

func TestConsoleAndFileStructuralParity(t *testing.T) {
	var consoleBuf bytes.Buffer
	consoleH := slog.NewJSONHandler(&prettyWriter{out: &consoleBuf}, &slog.HandlerOptions{
		AddSource: true,
	})
	fileH := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{
		AddSource: true,
	})

	logger := slog.New(newContextHandler(newMergedHandler(consoleH, fileH)))
	ctx := WithRequestID(context.Background(), "req-parity")
	logger.InfoContext(ctx, "parity check", "extra", "value")

	var m map[string]any
	if err := json.Unmarshal(consoleBuf.Bytes(), &m); err != nil {
		t.Fatalf("console output is not valid JSON: %v\nraw: %s", err, consoleBuf.String())
	}

	for _, key := range []string{"time", "level", "msg", "source", "request_id", "extra"} {
		if _, ok := m[key]; !ok {
			t.Errorf("console output missing key %q", key)
		}
	}
}

func TestMergedHandlerLevelFiltering(t *testing.T) {
	var consoleBuf, fileBuf bytes.Buffer

	consoleH := slog.NewJSONHandler(&consoleBuf, &slog.HandlerOptions{Level: slog.LevelWarn})
	fileH := slog.NewJSONHandler(&fileBuf, &slog.HandlerOptions{Level: slog.LevelInfo})

	logger := slog.New(newMergedHandler(consoleH, fileH))
	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg")

	consoleLines := nonEmptyLines(consoleBuf.String())
	if len(consoleLines) != 2 {
		t.Fatalf("console: expected 2 lines, got %d: %v", len(consoleLines), consoleLines)
	}

	fileLines := nonEmptyLines(fileBuf.String())
	if len(fileLines) != 3 {
		t.Fatalf("file: expected 3 lines, got %d: %v", len(fileLines), fileLines)
	}
}

func TestMergedHandlerEnabledOptimization(t *testing.T) {
	consoleH := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	fileH := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	merged := newMergedHandler(consoleH, fileH)

	if merged.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("DEBUG should be disabled when both sinks are WARN+")
	}
	if merged.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("INFO should be disabled when both sinks are WARN+")
	}
	if !merged.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("WARN should be enabled")
	}
	if !merged.Enabled(context.Background(), slog.LevelError) {
		t.Error("ERROR should be enabled")
	}
}

func TestContextHandlerAddsRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newContextHandler(slog.NewJSONHandler(&buf, nil)))

	ctx := WithRequestID(context.Background(), "req-42")
	logger.InfoContext(ctx, "with request id")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["request_id"] != "req-42" {
		t.Fatalf("expected request_id=req-42, got %v", m["request_id"])
	}
}

func TestContextHandlerAddsUserID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newContextHandler(slog.NewJSONHandler(&buf, nil)))

	ctx := WithUserID(context.Background(), "user-7")
	logger.InfoContext(ctx, "with user id")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["user_id"] != "user-7" {
		t.Fatalf("expected user_id=user-7, got %v", m["user_id"])
	}
}

func TestContextHandlerNoContextValues(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newContextHandler(slog.NewJSONHandler(&buf, nil)))

	logger.Info("no context values")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["request_id"]; ok {
		t.Fatal("should not contain request_id")
	}
	if _, ok := m["user_id"]; ok {
		t.Fatal("should not contain user_id")
	}
}

func TestDailyFileWriterRotation(t *testing.T) {
	dir := t.TempDir()
	w := newDailyFileWriter(dir)

	fakeDate := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	w.nowFn = func() time.Time { return fakeDate }

	_, _ = w.Write([]byte("line1\n"))
	_, _ = w.Write([]byte("line2\n"))

	fakeDate = fakeDate.AddDate(0, 0, 1)

	_, _ = w.Write([]byte("line3\n"))
	_ = w.Close()

	file1 := filepath.Join(dir, "2025_03_15.log")
	file2 := filepath.Join(dir, "2025_03_16.log")

	b1, err := os.ReadFile(file1)
	if err != nil {
		t.Fatalf("read file1: %v", err)
	}
	if string(b1) != "line1\nline2\n" {
		t.Fatalf("file1 content: %q", string(b1))
	}

	b2, err := os.ReadFile(file2)
	if err != nil {
		t.Fatalf("read file2: %v", err)
	}
	if string(b2) != "line3\n" {
		t.Fatalf("file2 content: %q", string(b2))
	}
}

func TestDailyFileWriterConcurrent(t *testing.T) {
	dir := t.TempDir()
	w := newDailyFileWriter(dir)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			msg := fmt.Sprintf("goroutine-%d\n", n)
			_, _ = w.Write([]byte(msg))
		}(i)
	}
	wg.Wait()

	_ = w.Flush()
	_ = w.Close()

	entries, _ := filepath.Glob(filepath.Join(dir, "*.log"))
	totalLines := 0
	for _, e := range entries {
		b, _ := os.ReadFile(e)
		totalLines += len(nonEmptyLines(string(b)))
	}
	if totalLines != 100 {
		t.Fatalf("expected 100 lines, got %d", totalLines)
	}
}

func TestDailyFileWriterFlush(t *testing.T) {
	dir := t.TempDir()
	w := newDailyFileWriter(dir)
	defer w.Close()

	_, _ = w.Write([]byte("buffered line\n"))
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	entries, _ := filepath.Glob(filepath.Join(dir, "*.log"))
	if len(entries) == 0 {
		t.Fatal("no log file after flush")
	}

	data, _ := os.ReadFile(entries[0])
	if !strings.Contains(string(data), "buffered line") {
		t.Errorf("flushed data not on disk: %q", string(data))
	}
}

func TestCloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	w := newDailyFileWriter(dir)
	_, _ = w.Write([]byte("hello\n"))

	if err := w.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestFlushPackageLevel(t *testing.T) {
	setup(t)

	Load()
	defer Close()

	slog.Info("flush test message")

	if err := Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	logsDir := filepath.Join(config.DataDir(), "logs")
	entries, _ := filepath.Glob(filepath.Join(logsDir, "*.log"))
	if len(entries) == 0 {
		t.Fatal("no log files after Flush")
	}

	data, _ := os.ReadFile(entries[0])
	if !strings.Contains(string(data), "flush test message") {
		t.Errorf("flushed message not on disk: %s", string(data))
	}
}

func TestFlushWhenNoWriter(t *testing.T) {
	setup(t)

	if err := Flush(); err != nil {
		t.Fatalf("Flush with no writer: %v", err)
	}
}

func TestPrettyWriter(t *testing.T) {
	var buf bytes.Buffer
	w := &prettyWriter{out: &buf}

	compact := []byte(`{"level":"INFO","msg":"hello","count":42}` + "\n")
	n, err := w.Write(compact)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(compact) {
		t.Fatalf("Write returned %d, want %d", n, len(compact))
	}

	got := buf.String()
	var m map[string]any
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, got)
	}
	if !strings.Contains(got, "  \"level\"") {
		t.Errorf("expected 2-space indented output, got:\n%s", got)
	}
	if len(nonEmptyLines(got)) < 3 {
		t.Errorf("expected multi-line output, got:\n%s", got)
	}
}

func TestPrettyWriterFallback(t *testing.T) {
	var buf bytes.Buffer
	w := &prettyWriter{out: &buf}

	notJSON := []byte("this is not json\n")
	n, err := w.Write(notJSON)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(notJSON) {
		t.Fatalf("Write returned %d, want %d", n, len(notJSON))
	}
	if buf.String() != string(notJSON) {
		t.Errorf("expected passthrough, got: %q", buf.String())
	}
}

func TestParseLevelVariants(t *testing.T) {
	cases := []struct {
		input string
		want  slog.Level
		err   bool
	}{
		{"DEBUG", slog.LevelDebug, false},
		{"info", slog.LevelInfo, false},
		{"Warn", slog.LevelWarn, false},
		{"WARNING", slog.LevelWarn, false},
		{"ERROR", slog.LevelError, false},
		{"  info  ", slog.LevelInfo, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			var lv slog.LevelVar
			err := parseLevel(&lv, tc.input)
			if tc.err {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if lv.Level() != tc.want {
				t.Errorf("got %v, want %v", lv.Level(), tc.want)
			}
		})
	}
}
