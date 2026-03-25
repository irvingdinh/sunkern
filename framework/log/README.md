# Logging

The `framework/log` package provides structured logging for Sunkern applications. It is built on Go's standard `log/slog` library and writes to two sinks simultaneously — console (stdout) and daily-rotated log files — so every log event is both visible in real-time and preserved on disk.

The package is intentionally **write-only**. It handles log output, context injection, and file lifecycle. Log querying, analysis, and management belong to external tools reading the JSONL files or consuming stdout.

```go
import (
    "log/slog"

    "sunkern.local/framework/config"
    "sunkern.local/framework/log"
)

func main() {
    config.Load()
    log.Load()
    slog.Info("application started")
}
```

After `log.Load()`, the standard `slog` package is ready to use. No custom logger instance is needed — the package configures the global `slog` default.

## Configuration

The logging package reads two config keys, both with sensible defaults:

| Key | Env Var | Default | Description |
|-----|---------|---------|-------------|
| `log.level` | `LOG_LEVEL` | `"INFO"` | Minimum log level for both sinks |
| `log.format` | `LOG_FORMAT` | `"json"` | Console output format |

Override via environment variables for quick adjustments:

```bash
LOG_LEVEL=DEBUG LOG_FORMAT=json-pretty go run .
```

> **Note:** The `log.format` setting controls console output only. File output is always compact JSONL regardless of this setting.

## Writing Log Messages

### Log Levels

The package supports four log levels, controlled by a single `log.level` that applies to both console and file output:

| Level | When to Use | Examples |
|-------|-------------|---------|
| `DEBUG` | Trace-level detail for diagnosing issues | Request payloads, SQL queries, internal state |
| `INFO` | Normal operational events | Server started, request handled, user created |
| `WARN` | Degraded but recoverable conditions | Retries, fallbacks, approaching rate limits |
| `ERROR` | Failures that need human attention | Unrecoverable errors, broken invariants |

```go
slog.Debug("executing query", "sql", q.String())
slog.Info("server listening", "port", 19110)
slog.Warn("rate limit approaching", "remaining", 5)
slog.Error("database connection failed", "error", err)
```

Reserve `ERROR` for genuine failures. Business-logic rejections like "user not found" are normal — use `INFO` or `WARN`, not `ERROR`.

### Structured Attributes

Always use key-value pairs for log data. Messages should be static strings that describe the event — put variable data in attributes, not in the message.

```go
// Correct: static message, structured attributes
slog.InfoContext(ctx, "user created", "user_id", u.ID, "email", u.Email)

// Incorrect: variable data in the message
slog.Info(fmt.Sprintf("user %s created with email %s", u.ID, u.Email))
```

This produces clean, parseable JSON:

```json
{"time":"...","level":"INFO","msg":"user created","user_id":"abc123","email":"user@example.com"}
```

### Context-Aware Logging

In all request-scoped code — handlers, middleware, services called from handlers — use the context-aware variants: `InfoContext`, `WarnContext`, `ErrorContext`, `DebugContext`. This ensures `request_id` and `user_id` are automatically included in every log entry.

```go
func (c *usersController) create(w http.ResponseWriter, r *http.Request) {
    // request_id and user_id are auto-injected from r.Context()
    slog.InfoContext(r.Context(), "user created", "email", u.Email)
}
```

Bare `slog.Info` / `slog.Error` (without context) should only be used during application boot, where no request context exists:

```go
func main() {
    config.Load()
    log.Load()
    slog.Info("application started", "port", config.Get[int]("http.port"))
}
```

## Context Values

The package provides two context injection functions. Values stored in context are automatically added to every log record produced from that context — you never need to pass them manually as key-value pairs.

### Request ID

Use `log.WithRequestID(ctx, id)` in request middleware to store the request ID. The context handler auto-injects it as a `request_id` field.

```go
func requestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := generateRequestID()
        ctx := log.WithRequestID(r.Context(), id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

To extract the request ID from context (e.g., for including in HTTP response headers):

```go
reqID := log.RequestIDFromCtx(r.Context())
w.Header().Set("X-Request-Id", reqID)
```

### User ID

Use `log.WithUserID(ctx, id)` in auth middleware after verifying the user's identity. It works identically to request ID — auto-injected as a `user_id` field.

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims, err := verifyToken(r)
        if err != nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        ctx := log.WithUserID(r.Context(), claims.UserID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

With both middleware in place, every log entry from a request handler automatically includes both fields:

```json
{
  "time": "2026-03-25T14:30:45.123Z",
  "level": "INFO",
  "msg": "order placed",
  "source": {"function": "...", "file": "orders.controller.go", "line": 42},
  "request_id": "req-abc123",
  "user_id": "usr-789",
  "order_id": "ord-456",
  "total": 99.99
}
```

## Architecture

### Handler Chain

The logger is assembled from three handler components:

```
slog.Default()
  └── contextHandler       ← extracts request_id, user_id from context
        └── mergedHandler   ← fans out to both sinks
              ├── console JSONHandler  (stdout)
              └── file JSONHandler     ({DATA_DIR}/logs/YYYY_MM_DD.log)
```

Both handlers have `AddSource: true` enabled, so every log entry includes the source file and line number.

### Console Output

Console output goes to stdout. The format is controlled by the `log.format` config key:

**`"json"` (default)** — compact single-line JSON, optimized for log aggregation systems:

```json
{"time":"2026-03-25T14:30:45.123Z","level":"INFO","source":{"function":"...","file":"users.controller.go","line":42},"msg":"user created","request_id":"req-abc123","user_id":"usr-789"}
```

**`"json-pretty"`** — 2-space indented JSON, optimized for human reading during development:

```json
{
  "time": "2026-03-25T14:30:45.123Z",
  "level": "INFO",
  "source": {
    "function": "...",
    "file": "users.controller.go",
    "line": 42
  },
  "msg": "user created",
  "request_id": "req-abc123",
  "user_id": "usr-789"
}
```

### File Output

Log files are written to `{DATA_DIR}/logs/` with daily rotation:

```
{DATA_DIR}/logs/
  2026_03_23.log
  2026_03_24.log
  2026_03_25.log    ← current day
```

Each file contains compact JSONL (one JSON object per line), regardless of the `log.format` setting. The file writer uses a 64 KB buffer with a 200 ms background flush interval, balancing write throughput with data freshness.

Rotation happens lazily — a new file is created on the first write after midnight.

## Lifecycle

### Initialization

`log.Load()` must be called after `config.Load()`. It:

1. Reads `log.level` and `log.format` from config (with defaults)
2. Creates the `{DATA_DIR}/logs/` directory if it does not exist
3. Instantiates the console handler (format-aware) and file handler (always compact JSONL)
4. Wraps both with the context handler for auto-injection
5. Sets the result as the `slog` default logger
6. Registers a container shutdown hook to flush and close the file writer

If `log.level` or `log.format` contain invalid values, `Load()` panics.

### Flush and Close

`log.Flush()` forces any buffered log data to be written to disk immediately. Use it before critical operations where you need to ensure logs are persisted:

```go
log.Flush()
```

`log.Close()` flushes buffered data and closes the file writer. It is automatically called during graceful shutdown via the container hook. Both functions are safe to call concurrently and multiple times (idempotent).

### Reloading

`log.Load()` can be called again to replace the logger with a new configuration. The previous file writer is automatically closed. This is useful in tests:

```go
func TestSomething(t *testing.T) {
    t.Setenv("DATA_DIR", t.TempDir())
    config.Load()
    log.Load()
    defer log.Close()
    // Test code with isolated logging
}
```

> **Warning:** Always set `DATA_DIR` to a temp directory in tests. Without it, test logs pollute the default `~/.standalone/logs/` directory.
