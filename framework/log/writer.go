package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// prettyWriter wraps an io.Writer and reformats each compact JSON blob into
// indented JSON (2-space indent) before writing. Used for console output so
// the developer can scan log entries visually. Non-JSON input passes through
// unchanged.
type prettyWriter struct {
	out io.Writer
}

func (w *prettyWriter) Write(p []byte) (int, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, bytes.TrimSpace(p), "", "  "); err != nil {
		return w.out.Write(p)
	}
	buf.WriteByte('\n')
	if _, err := w.out.Write(buf.Bytes()); err != nil {
		return 0, err
	}
	return len(p), nil
}

// dailyFileWriter is the file sink behind the JSON file handler: a
// thread-safe io.WriteCloser that appends bytes to one JSONL file per calendar
// day (YYYY_MM_DD.log) under a fixed directory. It is created when [Load] runs;
// write errors (including rotation failures) propagate to slog.
//
// Rotation happens on the first write after the date changes; the mutex
// serializes writes and protects the current file handle and date string.
type dailyFileWriter struct {
	mu    sync.Mutex
	dir   string
	date  string
	file  *os.File
	nowFn func() time.Time // injectable for testing
}

func newDailyFileWriter(dir string) *dailyFileWriter {
	return &dailyFileWriter{
		dir:   dir,
		nowFn: time.Now,
	}
}

// Write appends p to today's log file, rotating to a new file when the
// calendar day changes (under mu).
func (w *dailyFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	today := w.nowFn().Format("2006_01_02")
	if today != w.date {
		if err := w.rotate(today); err != nil {
			return 0, fmt.Errorf("log: rotate: %w", err)
		}
	}
	return w.file.Write(p)
}

// rotate closes the previous file (if any) and opens path for append in dir.
func (w *dailyFileWriter) rotate(date string) error {
	if w.file != nil {
		_ = w.file.Close()
	}
	path := filepath.Join(w.dir, date+".log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	w.date = date
	return nil
}

// Close flushes and closes the current file handle. Safe to call multiple
// times — subsequent calls return nil.
func (w *dailyFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		w.date = ""
		return err
	}
	return nil
}
