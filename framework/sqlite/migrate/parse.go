package migrate

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

// ParsedMigration holds the SQL statements extracted from a migration file.
type ParsedMigration struct {
	Up   []string
	Down []string
}

// Direction indicates whether a migration is being applied or rolled back.
type Direction int

const (
	DirectionUp Direction = iota
	DirectionDown
)

// Parse reads a migration file and splits it into Up and Down statement lists.
// The file must contain a "-- +sunkern Up" annotation. The "-- +sunkern Down"
// annotation is optional.
//
// Statements are split on semicolons, with proper handling of semicolons
// inside quoted strings and comments. Use "-- +sunkern StatementBegin" and
// "-- +sunkern StatementEnd" to wrap statements that contain embedded
// semicolons (e.g., triggers).
func Parse(r io.Reader) (ParsedMigration, error) {
	var result ParsedMigration
	var buf strings.Builder

	state := stateStart
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Check for annotation lines.
		if ann := parseAnnotation(line); ann != "" {
			switch ann {
			case "Up":
				if state != stateStart {
					return result, fmt.Errorf("line %d: duplicate or misplaced -- +sunkern Up", lineNum)
				}
				state = stateUp
				continue

			case "Down":
				if state == stateStart {
					return result, fmt.Errorf("line %d: -- +sunkern Down before Up", lineNum)
				}
				if state == stateUpBlock {
					return result, fmt.Errorf("line %d: -- +sunkern Down inside StatementBegin block", lineNum)
				}
				// Flush any remaining Up buffer.
				if s := strings.TrimSpace(buf.String()); s != "" {
					result.Up = append(result.Up, s)
				}
				buf.Reset()
				state = stateDown
				continue

			case "StatementBegin":
				switch state {
				case stateUp:
					state = stateUpBlock
				case stateDown:
					state = stateDownBlock
				default:
					return result, fmt.Errorf("line %d: StatementBegin outside Up/Down section", lineNum)
				}
				continue

			case "StatementEnd":
				switch state {
				case stateUpBlock:
					if s := strings.TrimSpace(buf.String()); s != "" {
						result.Up = append(result.Up, s)
					}
					buf.Reset()
					state = stateUp
				case stateDownBlock:
					if s := strings.TrimSpace(buf.String()); s != "" {
						result.Down = append(result.Down, s)
					}
					buf.Reset()
					state = stateDown
				default:
					return result, fmt.Errorf("line %d: StatementEnd without StatementBegin", lineNum)
				}
				continue
			}
		}

		// Skip lines before the first annotation.
		if state == stateStart {
			continue
		}

		// Inside a StatementBegin block: accumulate everything until
		// StatementEnd, ignoring semicolons.
		if state == stateUpBlock || state == stateDownBlock {
			if buf.Len() > 0 {
				buf.WriteByte('\n')
			}
			buf.WriteString(line)
			continue
		}

		// Normal mode: split on semicolons.
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isLineComment(trimmed) {
			continue
		}

		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(line)

		if endsWithSemicolon(line) {
			s := strings.TrimSpace(buf.String())
			if s != "" {
				switch state {
				case stateUp:
					result.Up = append(result.Up, s)
				case stateDown:
					result.Down = append(result.Down, s)
				}
			}
			buf.Reset()
		}
	}

	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("reading migration: %w", err)
	}

	if state == stateStart {
		return result, fmt.Errorf("missing -- +sunkern Up annotation")
	}

	if state == stateUpBlock || state == stateDownBlock {
		return result, fmt.Errorf("unterminated StatementBegin block")
	}

	// Flush any remaining buffer.
	if s := strings.TrimSpace(buf.String()); s != "" {
		switch state {
		case stateUp:
			result.Up = append(result.Up, s)
		case stateDown:
			result.Down = append(result.Down, s)
		}
	}

	return result, nil
}

// parseFilename extracts the version number and descriptive name from a
// migration filename. Expected format: "NNNNN_descriptive_name.sql"
// (e.g., "00001_create_users.sql" → version 1, name "create_users").
func parseFilename(name string) (version int, migrationName string, err error) {
	// Strip directory path and extension.
	base := filepath.Base(name)
	base = strings.TrimSuffix(base, ".sql")

	idx := strings.IndexByte(base, '_')
	if idx < 0 {
		return 0, "", fmt.Errorf("invalid migration filename %q: missing underscore separator", name)
	}

	v, err := strconv.Atoi(base[:idx])
	if err != nil {
		return 0, "", fmt.Errorf("invalid migration filename %q: version %q is not a number", name, base[:idx])
	}
	if v <= 0 {
		return 0, "", fmt.Errorf("invalid migration filename %q: version must be positive", name)
	}

	migrationName = base[idx+1:]
	if migrationName == "" {
		return 0, "", fmt.Errorf("invalid migration filename %q: missing name after version", name)
	}

	return v, migrationName, nil
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

type parserState int

const (
	stateStart parserState = iota
	stateUp
	stateUpBlock
	stateDown
	stateDownBlock
)

const annotationPrefix = "-- +sunkern "

// parseAnnotation extracts the annotation keyword from a line like
// "-- +sunkern Up". Returns "" if the line is not an annotation.
func parseAnnotation(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, annotationPrefix) {
		return ""
	}
	return strings.TrimSpace(trimmed[len(annotationPrefix):])
}

// isLineComment reports whether the entire line is a SQL comment.
func isLineComment(line string) bool {
	return strings.HasPrefix(line, "--")
}

// endsWithSemicolon reports whether the line ends with a semicolon,
// properly ignoring trailing comments and handling quoted strings.
//
// Examples:
//
//	"SELECT 1;"                     → true
//	"INSERT VALUES ('a;b');"        → true
//	"INSERT VALUES ('a;b')"         → false
//	"SELECT 1; -- comment"          → true
//	"VALUES ('it''s;here');"        → true
func endsWithSemicolon(line string) bool {
	inSingleQuote := false
	commentStart := -1
	lastSemicolon := -1

	for i := 0; i < len(line); i++ {
		ch := line[i]

		if inSingleQuote {
			if ch == '\'' {
				// Check for escaped quote ('').
				if i+1 < len(line) && line[i+1] == '\'' {
					i++ // skip the escaped quote
				} else {
					inSingleQuote = false
				}
			}
			continue
		}

		switch ch {
		case '\'':
			inSingleQuote = true
			commentStart = -1
		case '-':
			if i+1 < len(line) && line[i+1] == '-' {
				commentStart = i
				i = len(line) // skip rest of line
			}
		case ';':
			lastSemicolon = i
			commentStart = -1
		}
	}

	if lastSemicolon < 0 {
		return false
	}

	// If there's a comment, the semicolon must be before it.
	if commentStart >= 0 && lastSemicolon > commentStart {
		return false
	}

	// Check that everything after the last semicolon (before any comment)
	// is whitespace.
	end := len(line)
	if commentStart >= 0 {
		end = commentStart
	}
	for i := lastSemicolon + 1; i < end; i++ {
		if line[i] != ' ' && line[i] != '\t' {
			return false
		}
	}

	return true
}
