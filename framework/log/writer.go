package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// dailyFileWriter implements io.WriteCloser and rotates log files daily.
// Each day's logs go to a separate file named YYYY_MM_DD.log.
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
