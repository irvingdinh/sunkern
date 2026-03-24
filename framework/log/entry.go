package log

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Entry represents a single parsed log entry from a JSONL log file. Known
// fields (time, level, msg, source, request_id, user_id) are promoted to
// typed struct fields; everything else lives in [Entry.Extra].
type Entry struct {
	Time      time.Time       `json:"time"`
	Level     string          `json:"level"`
	Message   string          `json:"msg"`
	Source    *EntrySource    `json:"source,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
	UserID    string          `json:"user_id,omitempty"`
	Extra     map[string]any  `json:"extra,omitempty"`
}

// EntrySource is the source code location recorded by slog when AddSource is
// enabled.
type EntrySource struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// MarshalJSON produces a flat JSON object that merges known fields with
// [Entry.Extra], matching the original JSONL structure.
func (e Entry) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, 6+len(e.Extra))
	m["time"] = e.Time.Format(time.RFC3339Nano)
	m["level"] = e.Level
	m["msg"] = e.Message
	if e.Source != nil {
		m["source"] = e.Source
	}
	if e.RequestID != "" {
		m["request_id"] = e.RequestID
	}
	if e.UserID != "" {
		m["user_id"] = e.UserID
	}
	for k, v := range e.Extra {
		m[k] = v
	}
	return json.Marshal(m)
}

// knownKeys is the set of top-level JSON keys that map to typed Entry fields.
var knownKeys = map[string]bool{
	"time": true, "level": true, "msg": true, "source": true,
	"request_id": true, "user_id": true,
}

// parseEntry decodes a single JSONL line into an Entry.
func parseEntry(line []byte) (Entry, error) {
	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return Entry{}, err
	}

	var e Entry

	if t, ok := raw["time"].(string); ok {
		e.Time, _ = time.Parse(time.RFC3339Nano, t)
	}
	if l, ok := raw["level"].(string); ok {
		e.Level = l
	}
	if m, ok := raw["msg"].(string); ok {
		e.Message = m
	}
	if rid, ok := raw["request_id"].(string); ok {
		e.RequestID = rid
	}
	if uid, ok := raw["user_id"].(string); ok {
		e.UserID = uid
	}
	if src, ok := raw["source"].(map[string]any); ok {
		e.Source = &EntrySource{}
		if f, ok := src["function"].(string); ok {
			e.Source.Function = f
		}
		if f, ok := src["file"].(string); ok {
			e.Source.File = f
		}
		if l, ok := src["line"].(float64); ok {
			e.Source.Line = int(l)
		}
	}

	// Collect extra fields.
	for k, v := range raw {
		if !knownKeys[k] {
			if e.Extra == nil {
				e.Extra = make(map[string]any)
			}
			e.Extra[k] = v
		}
	}

	return e, nil
}

// ---------------------------------------------------------------------------
// Query
// ---------------------------------------------------------------------------

// QueryOptions configures log entry filtering and pagination for [Query].
type QueryOptions struct {
	Level     string // minimum level: "DEBUG", "INFO", "WARN", "ERROR" (empty = all)
	Search    string // case-insensitive substring match against the message
	RequestID string // exact match on request_id
	UserID    string // exact match on user_id
	Limit     int    // max entries to return (0 = unlimited)
	Offset    int    // skip first N matching entries
}

// levelRank maps level strings to ordered integers for comparison. Unknown
// levels rank below DEBUG so they are never filtered out.
func levelRank(s string) int {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return 0
	case "INFO":
		return 1
	case "WARN", "WARNING":
		return 2
	case "ERROR":
		return 3
	default:
		return -1
	}
}

// matchesFilter returns true when entry e satisfies all criteria in opts.
func matchesFilter(e Entry, opts QueryOptions) bool {
	if opts.Level != "" {
		if levelRank(e.Level) < levelRank(opts.Level) {
			return false
		}
	}
	if opts.Search != "" {
		if !strings.Contains(strings.ToLower(e.Message), strings.ToLower(opts.Search)) {
			return false
		}
	}
	if opts.RequestID != "" && e.RequestID != opts.RequestID {
		return false
	}
	if opts.UserID != "" && e.UserID != opts.UserID {
		return false
	}
	return true
}

// Query reads log entries from the file for the given date (e.g.
// "2025_03_15") and returns those matching opts. Entries are returned in
// chronological order (oldest first). The second return value is the total
// count of matching entries before Limit/Offset are applied.
func Query(date string, opts QueryOptions) ([]Entry, int, error) {
	f, err := OpenFile(date)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // up to 1 MB per line

	var entries []Entry
	total := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		entry, err := parseEntry(line)
		if err != nil {
			continue // skip malformed lines
		}

		if !matchesFilter(entry, opts) {
			continue
		}

		total++

		if opts.Offset > 0 && total <= opts.Offset {
			continue
		}

		if opts.Limit > 0 && len(entries) >= opts.Limit {
			continue // keep counting for total
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return entries, total, fmt.Errorf("log: scan %s: %w", date, err)
	}

	return entries, total, nil
}
