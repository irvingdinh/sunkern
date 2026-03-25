---
title: Framework Logging Package
impact: HIGH
impactDescription: incorrect logging loses observability or leaks sensitive data
tags: log, logging, slog, structured, context, request-id, user-id
---

## Framework Logging Package

**Package:** `sunkern.local/framework/log`

The log package is a write-only logging package built on Go's standard
`log/slog`. The framework calls `log.Load()` during boot, installs one global
logger, and modules log through `slog` directly.

The package has one concern:

- write structured logs to both console and daily JSONL files

The package does **not** own log querying, log browsing, sampling, or
admin-style APIs.

---

### Architecture

```text
slog.Info(...)
  |
  v
contextHandler     -- injects request_id, user_id from context
  |
  v
mergedHandler      -- fans out to both sinks
  |         |
  v         v
console              file
(json or json-pretty)  (compact JSONL)
```

**Console** (`os.Stdout`): Compact JSON by default; set `LOG_FORMAT=json-pretty`
for 2-space indented JSON during local development.

**File** (`{DATA_DIR}/logs/YYYY_MM_DD.log`): Compact JSONL, one line per entry,
rotated on first write after midnight.

Both sinks include source location via `AddSource: true`.

---

### API Reference

```go
// Context helpers — store values that auto-inject into all log entries.
log.WithRequestID(ctx context.Context, id string) context.Context
log.RequestIDFromCtx(ctx context.Context) string
log.WithUserID(ctx context.Context, id string) context.Context
log.UserIDFromCtx(ctx context.Context) string

// Lifecycle — framework-managed.
log.Load()        // creates handlers, sets slog default
log.Flush() error // flushes buffered file output
log.Close() error // flushes and closes file writer

```

All logging is done through **stdlib `log/slog`**. Do not wrap or re-export
logging functions from this package.

---

### Configuration

| Key | Env Var | Default | Description |
|-----|---------|---------|-------------|
| `log.level` | `LOG_LEVEL` | `"INFO"` | Shared minimum level for both console and file sinks |
| `log.format` | `LOG_FORMAT` | `"json"` | Console output format: `json` (compact) or `json-pretty` (indented) |

There is no separate `log.console.level`. File output is always compact JSONL
regardless of `log.format`.

---

### Correct Usage

**Basic structured logging:**

```go
slog.Info("user created", "user_id", user.ID, "email", user.Email)
slog.Error("payment failed", "error", err, "amount", amount)
```

**With context:**

```go
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    slog.InfoContext(ctx, "creating order", "product_id", productID)

    if err := h.service.Create(ctx, order); err != nil {
        slog.ErrorContext(ctx, "order creation failed", "error", err)
        return
    }
}
```

**Setting request and user IDs in middleware:**

```go
ctx := log.WithRequestID(r.Context(), requestID)
ctx = log.WithUserID(ctx, userID)
r = r.WithContext(ctx)
```

---

### Incorrect Usage

**Do not use `fmt.Println` or `log.Println`:**

```go
// WRONG
fmt.Println("something happened")
log.Println("error:", err)

// CORRECT
slog.Info("something happened")
slog.Error("operation failed", "error", err)
```

**Do not interpolate data into the message:**

```go
// WRONG
slog.Info(fmt.Sprintf("user %s created order %s", userID, orderID))

// CORRECT
slog.Info("order created", "user_id", userID, "order_id", orderID)
```

**Do not log without context in request-scoped code:**

```go
// WRONG
slog.Info("fetching user", "id", userID)

// CORRECT
slog.InfoContext(r.Context(), "fetching user", "id", userID)
```

**Do not call lifecycle functions from module code:**

```go
// WRONG
log.Load()
log.Close()
```

**Do not add read-side APIs here:**

```go
// WRONG: these concepts do not belong in framework/log.
log.Query(...)
log.ListFiles(...)
log.CleanOldFiles(...)
log.OnRotate(...)
```

---

### Log Level Semantics

| Level | When to use |
|-------|-------------|
| `DEBUG` | Detailed trace for development and diagnosis |
| `INFO` | Normal operations and business events |
| `WARN` | Degraded but recoverable behavior |
| `ERROR` | Failed work that needs attention |

---

### File Rotation

- Files are named `YYYY_MM_DD.log`
- Rotation happens lazily on the first write after midnight
- Files use `O_APPEND | O_CREATE | O_WRONLY`
- The `dailyFileWriter` is mutex-protected and safe for concurrent writes
- The log directory is `{DATA_DIR}/logs/`

---

### Testing Pattern

Tests should follow the same lifecycle model as the application and config:

```go
func TestSomething(t *testing.T) {
    t.Setenv("DATA_DIR", t.TempDir())
    container.Reset()
    config.Load()
    log.Load()
    defer log.Close()

    // exercise code that logs
}
```

Do not rely on a special `log.Reset()` helper.
