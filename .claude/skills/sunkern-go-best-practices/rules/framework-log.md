---
title: Framework Logging Package
impact: HIGH
impactDescription: incorrect logging loses observability, breaks log viewer, or leaks sensitive data
tags: log, logging, slog, structured, context, request-id, user-id
---

## Framework Logging Package

**Package:** `sunkern.local/framework/log`

The log package provides dual-output structured logging built on Go's standard `log/slog`. Both outputs produce JSON with identical structure — same fields, same data. The framework calls `log.Load()` automatically during app startup; modules just use `slog` directly.

---

### Architecture

```
slog.Info(...)
  |
  v
contextHandler     -- injects request_id, user_id from context
  |
  v
mergedHandler      -- fans out to both sinks
  |         |
  v         v
console    file
(pretty)   (compact JSONL)
```

**Console** (`os.Stdout`): Pretty-printed JSON with 2-space indentation. For human eyes during development.

**File** (`{DATA_DIR}/logs/YYYY_MM_DD.log`): Compact single-line JSONL. For the admin log viewer. One file per calendar day, rotated on first write after midnight.

Both have `AddSource: true` — every log entry includes `source.function`, `source.file`, and `source.line`.

---

### API Reference

```go
// Context helpers — store values that auto-inject into all log entries.
log.WithRequestID(ctx context.Context, id string) context.Context
log.RequestIDFromCtx(ctx context.Context) string
log.WithUserID(ctx context.Context, id string) context.Context
log.UserIDFromCtx(ctx context.Context) string

// Lifecycle — called by the framework, not by module code.
log.Load()            // creates handlers, sets slog default
log.Close() error     // flushes file writer (called via container shutdown hook)
log.Reset()           // test-only: discards all output, resets state
```

All logging is done through **stdlib `log/slog`** — the log package does not wrap or re-export slog functions.

---

### Configuration Keys

| Key | Env Var | Default | Description |
|-----|---------|---------|-------------|
| `log.level` | `LOG_LEVEL` | `"INFO"` | Minimum log level: DEBUG, INFO, WARN, ERROR |

No format configuration — both outputs are always JSON.

---

### Correct Usage Patterns

**Basic structured logging:**

```go
slog.Info("user created", "user_id", user.ID, "email", user.Email)
slog.Error("payment failed", "err", err, "amount", amount)
slog.Debug("cache hit", "key", cacheKey, "ttl", ttl)
```

**With context (request_id and user_id auto-injected):**

```go
// In an HTTP handler — ctx carries request_id from RequestID middleware
// and user_id from auth middleware.
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    slog.InfoContext(ctx, "creating order", "product_id", productID)
    // Output includes: "request_id": "abc-123", "user_id": "user-42"
    // automatically — no need to pass them as attributes.

    if err := h.service.Create(ctx, order); err != nil {
        slog.ErrorContext(ctx, "order creation failed", "err", err)
        return
    }
    slog.InfoContext(ctx, "order created", "order_id", order.ID)
}
```

**Setting context values in middleware:**

```go
// RequestID middleware (built into framework, already does this):
ctx := log.WithRequestID(r.Context(), requestID)
r = r.WithContext(ctx)

// Auth middleware (you write this):
ctx := log.WithUserID(r.Context(), authenticatedUserID)
r = r.WithContext(ctx)
```

**Changing log level at runtime:**

```go
func (m *AdminModule) Boot(ctx context.Context) error {
    lv := container.MustMake[*slog.LevelVar]()
    // Later, in an admin API handler:
    lv.Set(slog.LevelDebug) // all logs now include DEBUG
}
```

**Using slog.LogAttrs for performance-sensitive paths:**

```go
// LogAttrs avoids allocations from key-value pairs.
slog.LogAttrs(ctx, slog.LevelInfo, "request processed",
    slog.String("method", r.Method),
    slog.String("path", r.URL.Path),
    slog.Int("status", status),
    slog.Duration("latency", elapsed),
)
```

---

### Incorrect Usage Patterns

**Using fmt.Println or log.Println:**

```go
// WRONG: bypasses structured logging, not captured in log files,
// no request_id, no source location, not JSON.
fmt.Println("something happened")
log.Println("error:", err)

// CORRECT:
slog.Info("something happened")
slog.Error("operation failed", "err", err)
```

**String interpolation in log messages:**

```go
// WRONG: the message should be a static description, not contain data.
// Data belongs in structured attributes.
slog.Info(fmt.Sprintf("user %s created order %s", userID, orderID))

// CORRECT: static message, structured attributes.
slog.Info("order created", "user_id", userID, "order_id", orderID)
```

**Forgetting context — losing request_id and user_id:**

```go
// WRONG: slog.Info without context — no request_id or user_id in output.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    slog.Info("fetching user", "id", userID)
}

// CORRECT: use InfoContext with r.Context().
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    slog.InfoContext(r.Context(), "fetching user", "id", userID)
}
```

**Calling log.Load() or log.Close() from module code:**

```go
// WRONG: Load resets logger state. Close shuts down file writer.
// Both are managed by the framework lifecycle.
func (m *MyModule) Boot(ctx context.Context) error {
    log.Load()  // destroys existing logger setup
    log.Close() // closes file writer prematurely
}
```

**Logging sensitive data:**

```go
// WRONG: secrets, tokens, passwords in log output.
slog.Info("authenticating", "password", req.Password, "token", jwt)

// CORRECT: log identifiers, not secrets.
slog.Info("authenticating", "user_id", req.UserID)
```

---

### Log Level Semantics

| Level | When to use | Example |
|-------|------------|---------|
| `DEBUG` | Detailed trace for development. Verbose. Disabled in production by default. | `slog.Debug("cache lookup", "key", k, "hit", found)` |
| `INFO` | Normal operations. Business events. State transitions. | `slog.Info("order created", "order_id", id)` |
| `WARN` | Degraded but still functioning. Unexpected but recoverable. | `slog.Warn("retry attempt", "attempt", n, "err", err)` |
| `ERROR` | Needs attention. Failed operation that affects the user. | `slog.Error("payment failed", "err", err, "user_id", uid)` |

---

### Output Format

**Console output** (pretty JSON, 2-space indent):

```json
{
  "time": "2026-03-24T10:00:00.123456+07:00",
  "level": "INFO",
  "source": {
    "function": "sunkern.local/service/features/ordermod.(*Handler).CreateOrder",
    "file": "/path/to/ordermod/handler.go",
    "line": 42
  },
  "msg": "order created",
  "request_id": "abc-123",
  "user_id": "user-42",
  "order_id": "ord-789"
}
```

**File output** (compact JSONL, one line per entry):

```json
{"time":"2026-03-24T10:00:00.123456+07:00","level":"INFO","source":{"function":"sunkern.local/service/features/ordermod.(*Handler).CreateOrder","file":"/path/to/ordermod/handler.go","line":42},"msg":"order created","request_id":"abc-123","user_id":"user-42","order_id":"ord-789"}
```

Both contain the **exact same fields**. No information is hidden from either output.

---

### File Rotation

- Files are named `YYYY_MM_DD.log` (e.g., `2026_03_24.log`)
- Rotation happens lazily on the first write after midnight
- Files use `O_APPEND | O_CREATE | O_WRONLY` — safe for concurrent writes
- The `dailyFileWriter` is thread-safe (mutex-protected)
- Log directory: `{DATA_DIR}/logs/` (created automatically by `log.Load()`)

---

### Context Injection Detail

The `contextHandler` sits at the top of the handler chain. On every log call, it:

1. Extracts `request_id` from context via `log.RequestIDFromCtx(ctx)`
2. Extracts `user_id` from context via `log.UserIDFromCtx(ctx)`
3. Adds each as `slog.String` attribute **only if non-empty**
4. Delegates to the mergedHandler (which writes to both console and file)

If you log without context (`slog.Info(...)` instead of `slog.InfoContext(ctx, ...)`), the `request_id` and `user_id` fields are simply absent — not empty strings.

---

### Thread Safety

- All slog handlers are stateless and concurrent-safe
- The `dailyFileWriter` serializes writes via mutex
- The `mergedHandler` delegates to both sinks independently
- `log.Close()` is safe to call multiple times (idempotent)

---

### Testing Patterns

**Reset log state between tests:**

```go
func setup(t *testing.T) {
    t.Helper()
    t.Setenv("DATA_DIR", t.TempDir())
    container.Reset()
    log.Reset()     // sets slog default to discard handler
    config.Load()
}
```

**Capture and verify log output in tests:**

```go
var buf bytes.Buffer
handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{AddSource: true})
logger := slog.New(newContextHandler(handler))

ctx := log.WithRequestID(context.Background(), "test-req")
logger.InfoContext(ctx, "test message", "key", "value")

var m map[string]any
json.Unmarshal(buf.Bytes(), &m)
// Assert on m["request_id"], m["key"], m["source"], etc.
```
