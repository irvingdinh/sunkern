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

	logsDir := filepath.Join(config.Get[string]("data_dir"), "logs")
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

	logsDir := filepath.Join(config.Get[string]("data_dir"), "logs")
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

func TestLoadWithTextFormat(t *testing.T) {
	setup(t)
	t.Setenv("LOG_FORMAT", "text")
	config.Load()

	Load()
	defer Close()

	// File handler should still produce valid JSON regardless of console format.
	slog.Info("text format test")

	logsDir := filepath.Join(config.Get[string]("data_dir"), "logs")
	entries, _ := filepath.Glob(filepath.Join(logsDir, "*.log"))
	if len(entries) == 0 {
		t.Fatal("expected at least one log file")
	}

	data, err := os.ReadFile(entries[0])
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("file output is not valid JSON: %v", err)
	}
}

func TestLoadPanicsOnInvalidFormat(t *testing.T) {
	setup(t)
	t.Setenv("LOG_FORMAT", "yaml")
	config.Load()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid LOG_FORMAT")
		}
		if msg, ok := r.(string); ok && !strings.Contains(msg, "unknown format") {
			t.Fatalf("unexpected panic message: %s", msg)
		}
	}()
	Load()
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
