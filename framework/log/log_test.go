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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setup(t *testing.T) {
	t.Helper()
	t.Setenv("DATA_DIR", t.TempDir())
	container.Reset()
	Reset()
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

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

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

	// Logging should write to the file. Verify a file exists in DATA_DIR/logs.
	slog.Info("test message")

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
	config.Load() // reload to pick up env

	Load()
	defer Close()

	// Console level should be ERROR. File always captures everything.
	// We verify the LevelVar in the container reflects the config.
	lv, err := container.Make[*slog.LevelVar]()
	if err != nil {
		t.Fatalf("resolve LevelVar: %v", err)
	}
	if lv.Level() != slog.LevelError {
		t.Errorf("console level = %v, want ERROR", lv.Level())
	}
}

func TestLevelVarInContainer(t *testing.T) {
	setup(t)

	Load()
	defer Close()

	lv, err := container.Make[*slog.LevelVar]()
	if err != nil {
		t.Fatalf("resolve LevelVar: %v", err)
	}

	// Default is INFO.
	if lv.Level() != slog.LevelInfo {
		t.Errorf("initial level = %v, want INFO", lv.Level())
	}

	// Dynamic change.
	lv.Set(slog.LevelError)
	if lv.Level() != slog.LevelError {
		t.Errorf("changed level = %v, want ERROR", lv.Level())
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

func TestConsoleAndFileStructuralParity(t *testing.T) {
	setup(t)

	// Capture console output via a buffer.
	var consoleBuf bytes.Buffer
	consoleH := slog.NewJSONHandler(&prettyWriter{out: &consoleBuf}, &slog.HandlerOptions{
		AddSource: true,
	})
	fileH := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{
		AddSource: true,
	})

	// Use a temporary logger to avoid interfering with global state.
	merged := newMergedHandler(consoleH, fileH)
	logger := slog.New(newContextHandler(merged))

	ctx := WithRequestID(context.Background(), "req-parity")
	logger.InfoContext(ctx, "parity check", "extra", "value")

	// Parse the console output (pretty-printed JSON).
	var m map[string]any
	if err := json.Unmarshal(consoleBuf.Bytes(), &m); err != nil {
		t.Fatalf("console output is not valid JSON: %v\nraw: %s", err, consoleBuf.String())
	}

	// Both console and file handlers have AddSource, so "source" must be present.
	for _, key := range []string{"time", "level", "msg", "source", "request_id", "extra"} {
		if _, ok := m[key]; !ok {
			t.Errorf("console output missing key %q", key)
		}
	}

	// Verify source is a structured object (not just a string).
	src, ok := m["source"].(map[string]any)
	if !ok {
		t.Fatalf("source should be an object, got %T", m["source"])
	}
	for _, field := range []string{"function", "file", "line"} {
		if _, ok := src[field]; !ok {
			t.Errorf("source missing field %q", field)
		}
	}
}

// ---------------------------------------------------------------------------
// MergedHandler
// ---------------------------------------------------------------------------

func TestMergedHandlerLevelFiltering(t *testing.T) {
	var consoleBuf, fileBuf bytes.Buffer

	consoleH := slog.NewJSONHandler(&consoleBuf, &slog.HandlerOptions{Level: slog.LevelWarn})
	fileH := slog.NewJSONHandler(&fileBuf, &slog.HandlerOptions{Level: slog.LevelInfo})

	merged := newMergedHandler(consoleH, fileH)
	logger := slog.New(merged)

	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg")

	// Console (WARN+) should have warn and error only.
	consoleLines := nonEmptyLines(consoleBuf.String())
	if len(consoleLines) != 2 {
		t.Fatalf("console: expected 2 lines, got %d: %v", len(consoleLines), consoleLines)
	}

	// File (INFO+) should have info, warn, error.
	fileLines := nonEmptyLines(fileBuf.String())
	if len(fileLines) != 3 {
		t.Fatalf("file: expected 3 lines, got %d: %v", len(fileLines), fileLines)
	}
}

// ---------------------------------------------------------------------------
// ContextHandler
// ---------------------------------------------------------------------------

func TestContextHandlerAddsRequestID(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	ch := newContextHandler(inner)
	logger := slog.New(ch)

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
	inner := slog.NewJSONHandler(&buf, nil)
	ch := newContextHandler(inner)
	logger := slog.New(ch)

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
	inner := slog.NewJSONHandler(&buf, nil)
	ch := newContextHandler(inner)
	logger := slog.New(ch)

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

// ---------------------------------------------------------------------------
// DailyFileWriter
// ---------------------------------------------------------------------------

func TestDailyFileWriterRotation(t *testing.T) {
	dir := t.TempDir()
	w := newDailyFileWriter(dir)

	fakeDate := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	w.nowFn = func() time.Time { return fakeDate }

	_, _ = w.Write([]byte("line1\n"))
	_, _ = w.Write([]byte("line2\n"))

	// Advance clock to next day.
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
	defer w.Close()

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

// ---------------------------------------------------------------------------
// Close / Reset
// ---------------------------------------------------------------------------

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

func TestResetDiscardsOutput(t *testing.T) {
	setup(t)

	Load()

	Reset()

	// After Reset, logging should go nowhere. Verify no new log files are
	// created (the old ones from Load may exist, but no new data).
	tmpDir := t.TempDir()
	t.Setenv("DATA_DIR", tmpDir)

	slog.Info("this should be discarded")

	entries, _ := filepath.Glob(filepath.Join(tmpDir, "logs", "*.log"))
	if len(entries) != 0 {
		t.Fatalf("expected no log files after Reset, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// parseLevel
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// prettyWriter
// ---------------------------------------------------------------------------

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

	// Must be valid JSON.
	var m map[string]any
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, got)
	}

	// Must contain 2-space indentation.
	if !strings.Contains(got, "  \"level\"") {
		t.Errorf("expected 2-space indented output, got:\n%s", got)
	}

	// Must contain newlines (multi-line).
	lines := nonEmptyLines(got)
	if len(lines) < 3 {
		t.Errorf("expected multi-line output, got %d lines:\n%s", len(lines), got)
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

	// Non-JSON input should pass through unchanged.
	if buf.String() != string(notJSON) {
		t.Errorf("expected passthrough, got: %q", buf.String())
	}
}

// ---------------------------------------------------------------------------
// parseLevel
// ---------------------------------------------------------------------------

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
