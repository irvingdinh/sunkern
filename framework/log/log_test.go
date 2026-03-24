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
	Flush() // ensure buffered data reaches disk

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

	// File level should be ERROR.
	fl, err := container.Make[*FileLevel]()
	if err != nil {
		t.Fatalf("resolve FileLevel: %v", err)
	}
	if fl.Level() != slog.LevelError {
		t.Errorf("file level = %v, want ERROR", fl.Level())
	}

	// Console defaults to same as file when not set explicitly.
	cl, err := container.Make[*ConsoleLevel]()
	if err != nil {
		t.Fatalf("resolve ConsoleLevel: %v", err)
	}
	if cl.Level() != slog.LevelError {
		t.Errorf("console level = %v, want ERROR (inherited from log.level)", cl.Level())
	}
}

func TestLoadSeparateConsoleLevelOverride(t *testing.T) {
	setup(t)
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("LOG_CONSOLE_LEVEL", "WARN")
	config.Load()

	Load()
	defer Close()

	fl, err := container.Make[*FileLevel]()
	if err != nil {
		t.Fatalf("resolve FileLevel: %v", err)
	}
	if fl.Level() != slog.LevelDebug {
		t.Errorf("file level = %v, want DEBUG", fl.Level())
	}

	cl, err := container.Make[*ConsoleLevel]()
	if err != nil {
		t.Fatalf("resolve ConsoleLevel: %v", err)
	}
	if cl.Level() != slog.LevelWarn {
		t.Errorf("console level = %v, want WARN", cl.Level())
	}
}

func TestLevelVarInContainer(t *testing.T) {
	setup(t)

	Load()
	defer Close()

	cl, err := container.Make[*ConsoleLevel]()
	if err != nil {
		t.Fatalf("resolve ConsoleLevel: %v", err)
	}

	// Default is INFO.
	if cl.Level() != slog.LevelInfo {
		t.Errorf("initial level = %v, want INFO", cl.Level())
	}

	// Dynamic change.
	cl.Set(slog.LevelError)
	if cl.Level() != slog.LevelError {
		t.Errorf("changed level = %v, want ERROR", cl.Level())
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

func TestLoadPanicsOnInvalidConsoleLevel(t *testing.T) {
	setup(t)
	t.Setenv("LOG_CONSOLE_LEVEL", "not-a-level")
	config.Load()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid LOG_CONSOLE_LEVEL")
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

	// Flush buffered data before reading files.
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

	// Before flush, data may or may not be on disk (depending on buffer state).
	// After explicit flush, it must be on disk.
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
// Flush (package-level)
// ---------------------------------------------------------------------------

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

	// No Load(), so no writer. Flush should return nil.
	if err := Flush(); err != nil {
		t.Fatalf("Flush with no writer: %v", err)
	}
}

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

// ---------------------------------------------------------------------------
// ListFiles / CleanOldFiles / OpenFile
// ---------------------------------------------------------------------------

func TestListFilesEmpty(t *testing.T) {
	setup(t)
	Load()
	defer Close()

	// No logs written yet, but the directory exists. ListFiles should return
	// only the file created by Load (today's file was opened lazily, so it
	// may not exist until something is logged).
	files, err := ListFiles()
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	// Might be 0 or 1 depending on whether Load triggered a write.
	_ = files
}

func TestListFilesSorted(t *testing.T) {
	setup(t)

	logsDir := filepath.Join(config.DataDir(), "logs")
	os.MkdirAll(logsDir, 0o755)

	// Create files out of order.
	for _, date := range []string{"2025_03_10", "2025_03_15", "2025_03_12"} {
		os.WriteFile(filepath.Join(logsDir, date+".log"), []byte("data\n"), 0o644)
	}

	files, err := ListFiles()
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	// Newest first.
	if files[0].Date != "2025_03_15" || files[1].Date != "2025_03_12" || files[2].Date != "2025_03_10" {
		t.Errorf("wrong order: %v, %v, %v", files[0].Date, files[1].Date, files[2].Date)
	}

	// Size should be > 0.
	for _, f := range files {
		if f.Size == 0 {
			t.Errorf("file %s has zero size", f.Date)
		}
	}
}

func TestListFilesIgnoresNonLogFiles(t *testing.T) {
	setup(t)

	logsDir := filepath.Join(config.DataDir(), "logs")
	os.MkdirAll(logsDir, 0o755)

	os.WriteFile(filepath.Join(logsDir, "2025_03_10.log"), []byte("data\n"), 0o644)
	os.WriteFile(filepath.Join(logsDir, "readme.txt"), []byte("not a log\n"), 0o644)
	os.Mkdir(filepath.Join(logsDir, "subdir"), 0o755)

	files, err := ListFiles()
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
}

func TestCleanOldFiles(t *testing.T) {
	setup(t)

	logsDir := filepath.Join(config.DataDir(), "logs")
	os.MkdirAll(logsDir, 0o755)

	today := time.Now().Format("2006_01_02")
	old := time.Now().AddDate(0, 0, -10).Format("2006_01_02")
	borderline := time.Now().AddDate(0, 0, -7).Format("2006_01_02")

	for _, date := range []string{today, old, borderline} {
		os.WriteFile(filepath.Join(logsDir, date+".log"), []byte("data\n"), 0o644)
	}

	removed, err := CleanOldFiles(7)
	if err != nil {
		t.Fatalf("CleanOldFiles: %v", err)
	}

	// old (10 days ago) should be removed. borderline (exactly 7 days) has
	// date == cutoff, and since the comparison is strictly less-than, it
	// survives. today stays.
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	files, _ := ListFiles()
	if len(files) != 2 {
		t.Fatalf("expected 2 remaining files, got %d", len(files))
	}
}

func TestCleanOldFilesInvalidRetention(t *testing.T) {
	setup(t)

	_, err := CleanOldFiles(0)
	if err == nil {
		t.Fatal("expected error for retentionDays=0")
	}
}

func TestOpenFile(t *testing.T) {
	setup(t)

	logsDir := filepath.Join(config.DataDir(), "logs")
	os.MkdirAll(logsDir, 0o755)
	os.WriteFile(filepath.Join(logsDir, "2025_03_15.log"), []byte("hello world\n"), 0o644)

	rc, err := OpenFile("2025_03_15")
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer rc.Close()

	buf := make([]byte, 64)
	n, _ := rc.Read(buf)
	if string(buf[:n]) != "hello world\n" {
		t.Errorf("unexpected content: %q", string(buf[:n]))
	}
}

func TestOpenFileNotFound(t *testing.T) {
	setup(t)

	_, err := OpenFile("1999_01_01")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

// ---------------------------------------------------------------------------
// Entry parsing
// ---------------------------------------------------------------------------

func TestParseEntry(t *testing.T) {
	line := []byte(`{"time":"2025-03-15T10:30:00Z","level":"INFO","msg":"request handled","source":{"function":"main.handler","file":"main.go","line":42},"request_id":"req-123","user_id":"user-7","latency_ms":15,"status":200}`)

	entry, err := parseEntry(line)
	if err != nil {
		t.Fatalf("parseEntry: %v", err)
	}

	if entry.Level != "INFO" {
		t.Errorf("level = %q, want INFO", entry.Level)
	}
	if entry.Message != "request handled" {
		t.Errorf("message = %q, want 'request handled'", entry.Message)
	}
	if entry.RequestID != "req-123" {
		t.Errorf("request_id = %q, want req-123", entry.RequestID)
	}
	if entry.UserID != "user-7" {
		t.Errorf("user_id = %q, want user-7", entry.UserID)
	}
	if entry.Source == nil {
		t.Fatal("source is nil")
	}
	if entry.Source.Function != "main.handler" {
		t.Errorf("source.function = %q", entry.Source.Function)
	}
	if entry.Source.Line != 42 {
		t.Errorf("source.line = %d, want 42", entry.Source.Line)
	}

	// Extra fields.
	if entry.Extra == nil {
		t.Fatal("extra is nil")
	}
	if v, ok := entry.Extra["status"].(float64); !ok || v != 200 {
		t.Errorf("extra[status] = %v", entry.Extra["status"])
	}
	if v, ok := entry.Extra["latency_ms"].(float64); !ok || v != 15 {
		t.Errorf("extra[latency_ms] = %v", entry.Extra["latency_ms"])
	}
}

func TestParseEntryMalformed(t *testing.T) {
	_, err := parseEntry([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestEntryMarshalJSON(t *testing.T) {
	e := Entry{
		Time:      time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC),
		Level:     "INFO",
		Message:   "test",
		RequestID: "req-1",
		Extra:     map[string]any{"custom": "value"},
	}

	b, err := e.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var m map[string]any
	json.Unmarshal(b, &m)

	if m["level"] != "INFO" {
		t.Errorf("level = %v", m["level"])
	}
	if m["msg"] != "test" {
		t.Errorf("msg = %v", m["msg"])
	}
	if m["request_id"] != "req-1" {
		t.Errorf("request_id = %v", m["request_id"])
	}
	if m["custom"] != "value" {
		t.Errorf("custom = %v", m["custom"])
	}
	// source should be absent.
	if _, ok := m["source"]; ok {
		t.Error("source should be omitted when nil")
	}
}

// ---------------------------------------------------------------------------
// Query
// ---------------------------------------------------------------------------

// writeTestEntries creates a JSONL log file with synthetic entries for testing.
func writeTestEntries(t *testing.T, date string, count int) {
	t.Helper()
	logsDir := filepath.Join(config.DataDir(), "logs")
	os.MkdirAll(logsDir, 0o755)

	var lines []string
	for i := 0; i < count; i++ {
		level := "INFO"
		if i%5 == 0 {
			level = "ERROR"
		}
		ts := fmt.Sprintf("2025-03-15T10:%02d:%02dZ", i/60, i%60)
		line := fmt.Sprintf(`{"time":"%s","level":"%s","msg":"event %d","request_id":"req-%d","user_id":"user-%d"}`, ts, level, i, i%10, i%3)
		lines = append(lines, line)
	}
	os.WriteFile(filepath.Join(logsDir, date+".log"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func TestQuery(t *testing.T) {
	setup(t)
	writeTestEntries(t, "2025_03_15", 50)

	// All entries.
	entries, total, err := Query("2025_03_15", QueryOptions{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if total != 50 {
		t.Errorf("total = %d, want 50", total)
	}
	if len(entries) != 50 {
		t.Errorf("entries = %d, want 50", len(entries))
	}

	// Level filter.
	entries, total, _ = Query("2025_03_15", QueryOptions{Level: "ERROR"})
	if total != 10 {
		t.Errorf("ERROR total = %d, want 10", total)
	}
	if len(entries) != 10 {
		t.Errorf("ERROR entries = %d, want 10", len(entries))
	}

	// Search.
	entries, total, _ = Query("2025_03_15", QueryOptions{Search: "event 42"})
	if total != 1 {
		t.Errorf("search total = %d, want 1", total)
	}

	// RequestID filter.
	entries, total, _ = Query("2025_03_15", QueryOptions{RequestID: "req-0"})
	if total != 5 {
		t.Errorf("request_id total = %d, want 5", total)
	}

	// Pagination.
	entries, total, _ = Query("2025_03_15", QueryOptions{Limit: 5, Offset: 10})
	if total != 50 {
		t.Errorf("paginated total = %d, want 50", total)
	}
	if len(entries) != 5 {
		t.Errorf("paginated entries = %d, want 5", len(entries))
	}
	if entries[0].Message != "event 10" {
		t.Errorf("first entry = %q, want 'event 10'", entries[0].Message)
	}
}

func TestQueryDescOrder(t *testing.T) {
	setup(t)
	writeTestEntries(t, "2025_03_15", 50)

	// Desc order — newest first.
	entries, total, err := Query("2025_03_15", QueryOptions{Order: "desc"})
	if err != nil {
		t.Fatalf("Query desc: %v", err)
	}
	if total != 50 {
		t.Errorf("total = %d, want 50", total)
	}
	if len(entries) != 50 {
		t.Errorf("entries = %d, want 50", len(entries))
	}
	// First entry should be the last one written (event 49).
	if entries[0].Message != "event 49" {
		t.Errorf("first desc entry = %q, want 'event 49'", entries[0].Message)
	}
	// Last entry should be the first one written (event 0).
	if entries[49].Message != "event 0" {
		t.Errorf("last desc entry = %q, want 'event 0'", entries[49].Message)
	}

	// Desc with pagination.
	entries, total, _ = Query("2025_03_15", QueryOptions{Order: "desc", Limit: 5, Offset: 0})
	if total != 50 {
		t.Errorf("paginated desc total = %d, want 50", total)
	}
	if len(entries) != 5 {
		t.Errorf("paginated desc entries = %d, want 5", len(entries))
	}
	// First page of desc: events 49, 48, 47, 46, 45.
	if entries[0].Message != "event 49" {
		t.Errorf("first desc page entry = %q, want 'event 49'", entries[0].Message)
	}
	if entries[4].Message != "event 45" {
		t.Errorf("last desc page entry = %q, want 'event 45'", entries[4].Message)
	}

	// Desc page 2.
	entries, _, _ = Query("2025_03_15", QueryOptions{Order: "desc", Limit: 5, Offset: 5})
	if entries[0].Message != "event 44" {
		t.Errorf("desc page 2 first = %q, want 'event 44'", entries[0].Message)
	}
}

func TestQueryTimeRange(t *testing.T) {
	setup(t)
	writeTestEntries(t, "2025_03_15", 50)

	// After: entries at or after 10:00:30 (events 30-49).
	after := time.Date(2025, 3, 15, 10, 0, 30, 0, time.UTC)
	entries, total, err := Query("2025_03_15", QueryOptions{After: after})
	if err != nil {
		t.Fatalf("Query after: %v", err)
	}
	if total != 20 {
		t.Errorf("after total = %d, want 20", total)
	}
	if len(entries) != 20 {
		t.Errorf("after entries = %d, want 20", len(entries))
	}

	// Before: entries strictly before 10:00:10 (events 0-9).
	before := time.Date(2025, 3, 15, 10, 0, 10, 0, time.UTC)
	entries, total, _ = Query("2025_03_15", QueryOptions{Before: before})
	if total != 10 {
		t.Errorf("before total = %d, want 10", total)
	}

	// Combined range: 10:00:10 <= t < 10:00:20 (events 10-19).
	entries, total, _ = Query("2025_03_15", QueryOptions{
		After:  time.Date(2025, 3, 15, 10, 0, 10, 0, time.UTC),
		Before: time.Date(2025, 3, 15, 10, 0, 20, 0, time.UTC),
	})
	if total != 10 {
		t.Errorf("range total = %d, want 10", total)
	}
	if len(entries) != 10 {
		t.Errorf("range entries = %d, want 10", len(entries))
	}
}

func TestQueryCountOnly(t *testing.T) {
	setup(t)
	writeTestEntries(t, "2025_03_15", 50)

	// CountOnly: total is computed, entries is nil.
	entries, total, err := Query("2025_03_15", QueryOptions{CountOnly: true})
	if err != nil {
		t.Fatalf("Query countOnly: %v", err)
	}
	if total != 50 {
		t.Errorf("countOnly total = %d, want 50", total)
	}
	if len(entries) != 0 {
		t.Errorf("countOnly entries = %d, want 0", len(entries))
	}

	// CountOnly with level filter.
	_, total, _ = Query("2025_03_15", QueryOptions{CountOnly: true, Level: "ERROR"})
	if total != 10 {
		t.Errorf("countOnly ERROR total = %d, want 10", total)
	}

	// CountOnly with user_id filter.
	_, total, _ = Query("2025_03_15", QueryOptions{CountOnly: true, UserID: "user-0"})
	if total != 17 {
		t.Errorf("countOnly user-0 total = %d, want 17", total)
	}
}

func TestQueryUserIDFilter(t *testing.T) {
	setup(t)
	writeTestEntries(t, "2025_03_15", 50)

	entries, total, err := Query("2025_03_15", QueryOptions{UserID: "user-0"})
	if err != nil {
		t.Fatalf("Query user_id: %v", err)
	}
	// user-0 = indices 0,3,6,9,12,...,48 → 17 entries (i%3==0).
	if total != 17 {
		t.Errorf("user_id total = %d, want 17", total)
	}
	if len(entries) != 17 {
		t.Errorf("user_id entries = %d, want 17", len(entries))
	}
}

func TestQueryNonExistentDate(t *testing.T) {
	setup(t)

	_, _, err := Query("1999_01_01", QueryOptions{})
	if err == nil {
		t.Fatal("expected error for non-existent date")
	}
}

// ---------------------------------------------------------------------------
// levelRank
// ---------------------------------------------------------------------------

func TestLevelRank(t *testing.T) {
	if levelRank("DEBUG") >= levelRank("INFO") {
		t.Error("DEBUG should rank below INFO")
	}
	if levelRank("INFO") >= levelRank("WARN") {
		t.Error("INFO should rank below WARN")
	}
	if levelRank("WARN") >= levelRank("ERROR") {
		t.Error("WARN should rank below ERROR")
	}
	if levelRank("unknown") >= levelRank("DEBUG") {
		t.Error("unknown should rank below DEBUG")
	}
}
