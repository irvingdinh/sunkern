package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sunkern.local/framework/config"
)

// LogFile describes a daily log file on disk.
type LogFile struct {
	Date string // date portion of the filename, e.g. "2025_03_15"
	Path string // absolute filesystem path
	Size int64  // file size in bytes
}

// ListFiles returns all log files in the log directory sorted by date
// (newest first). Returns nil with no error when the directory does not exist
// or contains no log files.
func ListFiles() ([]LogFile, error) {
	dir := filepath.Join(config.DataDir(), "logs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("log: list files: %w", err)
	}

	var files []LogFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue // skip unreadable entries
		}
		date := strings.TrimSuffix(e.Name(), ".log")
		files = append(files, LogFile{
			Date: date,
			Path: filepath.Join(dir, e.Name()),
			Size: info.Size(),
		})
	}

	// Newest first — date strings sort lexicographically.
	sort.Slice(files, func(i, j int) bool {
		return files[i].Date > files[j].Date
	})

	return files, nil
}

// CleanOldFiles removes log files older than retentionDays from the log
// directory. Returns the number of files removed. A retentionDays value of 7
// means files from 8+ days ago are deleted; today and the last 7 days are
// kept.
func CleanOldFiles(retentionDays int) (int, error) {
	if retentionDays < 1 {
		return 0, fmt.Errorf("log: retention days must be >= 1")
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format("2006_01_02")

	files, err := ListFiles()
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, f := range files {
		if f.Date < cutoff {
			if err := os.Remove(f.Path); err != nil && !os.IsNotExist(err) {
				return removed, fmt.Errorf("log: remove %s: %w", f.Path, err)
			}
			removed++
		}
	}

	return removed, nil
}

// OpenFile opens the log file for the given date (e.g. "2025_03_15") for
// reading. The caller must close the returned reader when done.
func OpenFile(date string) (io.ReadCloser, error) {
	dir := filepath.Join(config.DataDir(), "logs")
	path := filepath.Join(dir, date+".log")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("log: open %s: %w", date, err)
	}
	return f, nil
}
