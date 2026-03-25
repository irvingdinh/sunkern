---
title: Framework Logging Package
impact: HIGH
impactDescription: incorrect logging loses observability or leaks sensitive data
tags: log, logging, slog, structured, context, request-id, user-id
---

## Framework Logging Package

**Location:** `framework/log/`

The `framework/log` package provides write-only structured logging built on Go's standard `log/slog`. It outputs to two sinks simultaneously: console (stdout) and daily-rotated JSONL files. Request and user IDs are auto-injected from context into every log record.

---

### Use Standard Library slog

Always use `log/slog` for logging. Never use `fmt.Println`, `log.Println`, the standard `log` package, or third-party loggers. After `log.Load()`, the `slog` default logger is configured with dual output and context injection — use it directly.

**Incorrect:**

```go
fmt.Println("Error:", err)
log.Printf("request failed: %v", err)
```

**Correct:**

```go
slog.Error("request failed", "error", err)
slog.InfoContext(ctx, "user created", "user_id", u.ID)
```

---

### Always Pass Context

Use `InfoContext`, `WarnContext`, `ErrorContext`, and `DebugContext` in all request-scoped code — handlers, middleware, services called from handlers. This propagates `request_id` and `user_id` automatically via the context handler.

Bare `slog.Info` / `slog.Error` (without context) are only appropriate during application boot, where no request context exists.

**Incorrect (bare slog call inside an HTTP handler):**

```go
func (c *usersController) create(w http.ResponseWriter, r *http.Request) {
    // Missing context — request_id and user_id will NOT appear in logs.
    slog.Info("user created", "email", u.Email)
}
```

**Correct (context-aware logging in an HTTP handler):**

```go
func (c *usersController) create(w http.ResponseWriter, r *http.Request) {
    slog.InfoContext(r.Context(), "user created", "email", u.Email)
    // Output includes request_id and user_id automatically.
}
```

---

### Structured Attributes

Use key-value pairs or `slog.Attr` for log data. Messages should be static strings describing the event — never use `fmt.Sprintf` or string interpolation in the message.

**Incorrect (interpolated message):**

```go
slog.InfoContext(ctx, fmt.Sprintf("user %s created with email %s", u.ID, u.Email))
```

**Incorrect (data embedded in message string):**

```go
slog.ErrorContext(ctx, "failed to create user "+u.Email+": "+err.Error())
```

**Correct (static message with structured key-value pairs):**

```go
slog.InfoContext(ctx, "user created", "user_id", u.ID, "email", u.Email)
slog.ErrorContext(ctx, "user creation failed", "email", u.Email, "error", err)
```

---

### Level Semantics

Choose log levels based on what happened, not how important it feels.

| Level | When to Use | Examples |
|-------|-------------|---------|
| `DEBUG` | Trace-level detail for diagnosing issues | Request payloads, SQL queries, internal state |
| `INFO` | Normal operational events | Server started, request handled, user created |
| `WARN` | Degraded but recoverable conditions | Retries, fallbacks, approaching rate limits |
| `ERROR` | Failures that need human attention | Unrecoverable errors, broken invariants, panics |

**Incorrect (ERROR for expected business-logic rejections):**

```go
// "Not found" is a normal outcome, not a failure.
slog.ErrorContext(ctx, "user not found", "id", id)
```

**Incorrect (INFO for failures that need attention):**

```go
// Database connection failure needs human attention — not INFO.
slog.InfoContext(ctx, "database connection failed", "error", err)
```

**Correct (appropriate level for the situation):**

```go
slog.DebugContext(ctx, "executing query", "sql", q.String())
slog.InfoContext(ctx, "user created", "user_id", u.ID)
slog.WarnContext(ctx, "rate limit approaching", "remaining", 5)
slog.ErrorContext(ctx, "database connection failed", "error", err)
```

---

### Console Format

The `log.format` config key (env `LOG_FORMAT`) controls console output format only. File output is always compact JSONL regardless of this setting.

| Value | Effect | Use Case |
|-------|--------|----------|
| `"json"` (default) | Compact single-line JSON to stdout | Production, log aggregation |
| `"json-pretty"` | 2-space indented JSON to stdout | Local development |

**Incorrect (inventing a separate file format key):**

```go
// These keys do not exist and should not be created.
config.SetDefault("log.file_format", "json")
config.SetDefault("log.console_format", "json-pretty")
```

**Correct (using the single format key for console):**

```go
config.SetDefault("log.format", "json")
// Override for development: LOG_FORMAT=json-pretty go run .
```

---

### Dual Output

The logger writes to two sinks simultaneously: console (stdout) and file (`{DATA_DIR}/logs/YYYY_MM_DD.log`). Both produce identical JSON structure with source location. Do not add custom sinks or modify the handler pipeline.

**Incorrect (adding a third sink or custom handler):**

```go
// Do not replace the slog default or wrap it with additional handlers.
customHandler := slog.NewTextHandler(os.Stderr, nil)
slog.SetDefault(slog.New(customHandler))
```

**Correct (relying on the built-in dual output):**

```go
log.Load() // Sets up console + file sinks as the slog default.
slog.InfoContext(ctx, "event happened")
// Writes to both stdout and {DATA_DIR}/logs/2026_03_25.log
```

---

### Single Level

One shared `log.level` controls both console and file output. Do not invent per-sink level configuration. Both sinks should see the same events for consistent observability.

**Incorrect (inventing split-level configuration):**

```go
// These keys do not exist and should not be created.
config.SetDefault("log.console_level", "INFO")
config.SetDefault("log.file_level", "DEBUG")
```

**Correct (single level for both sinks):**

```go
config.SetDefault("log.level", "INFO")
// Both console and file filter at the same level.
// Override at runtime: LOG_LEVEL=DEBUG go run .
```

---

### Request ID

Use `log.WithRequestID(ctx, id)` in request middleware to store the request ID in context. The `contextHandler` auto-injects it as a `request_id` field into every log record created from that context. Do not manually add `"request_id"` as a key-value pair.

**Incorrect (manually adding request_id to every log call):**

```go
slog.InfoContext(ctx, "processing request", "request_id", reqID, "path", r.URL.Path)
slog.InfoContext(ctx, "request complete", "request_id", reqID, "status", 200)
// Tedious, error-prone, easy to forget in some calls.
```

**Correct (storing once in middleware, auto-injected everywhere):**

```go
// In middleware:
ctx := log.WithRequestID(r.Context(), generateID())

// In any handler or service using this context:
slog.InfoContext(ctx, "processing request", "path", r.URL.Path)
// Output: {"msg":"processing request","request_id":"req-abc123","path":"/api/users",...}
```

---

### User ID

Use `log.WithUserID(ctx, id)` in auth middleware to store the authenticated user's ID in context. It works identically to request ID — auto-injected into all log records from that context.

**Incorrect (manually adding user_id to every log call):**

```go
slog.InfoContext(ctx, "action performed", "user_id", userID, "action", "delete")
```

**Correct (storing once in auth middleware, auto-injected everywhere):**

```go
// In auth middleware, after verifying the token:
ctx = log.WithUserID(ctx, claims.UserID)

// In any downstream handler:
slog.InfoContext(ctx, "action performed", "action", "delete")
// Output includes user_id automatically alongside request_id.
```

---

### Write-Only Surface

The `framework/log` package is strictly write-only. Do not add query APIs, file management, log sampling, rotation configuration, or admin endpoints. Log analysis belongs to external tools reading the JSONL files or consuming stdout.

**Incorrect (adding query or management APIs to the log package):**

```go
// None of these exist or should be created.
log.Query(log.Filter{Level: "ERROR", Since: time.Now().Add(-1*time.Hour)})
log.Rotate()
log.SetSamplingRate(0.1)
log.SetMaxFileSize(100 * 1024 * 1024)
log.RegisterAdminEndpoint(mux)
```

**Correct (keeping the package limited to its write-only surface):**

```go
// The complete public API:
log.Load()                          // Initialize dual-output logger
log.Flush()                         // Force buffered data to disk
log.Close()                         // Flush and close file writer
log.WithRequestID(ctx, id)          // Store request ID in context
log.RequestIDFromCtx(ctx)           // Extract request ID from context
log.WithUserID(ctx, id)             // Store user ID in context
log.UserIDFromCtx(ctx)              // Extract user ID from context
// Everything else: use slog directly.
```
