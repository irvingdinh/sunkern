# Logging

## Introduction

The `framework/log` package provides process-wide structured logging for a
Sunkern application.

It is responsible for:

- creating the global `slog` logger used by the framework and service
- writing every log entry to both console and file outputs
- injecting `request_id` and `user_id` from `context.Context`
- rotating file output daily under `{DATA_DIR}/logs`
- exposing one shared runtime log level through the container

The package is intentionally narrow. It is a write-only logging package. It
does not own log querying, log browsing, log retention commands, sampling, or
admin-style APIs.

## Logging Outputs

Sunkern writes each log entry to two sinks:

1. Console output
2. File output

Console output is pretty-printed JSON for human readability during local
development.

File output is compact JSONL for durable machine-readable logs.

Both outputs contain the same structured fields. Only formatting differs.

## File Location

Log files live under:

```text
{DATA_DIR}/logs/
  YYYY_MM_DD.log
```

For example:

```text
/tmp/sunkern_data_xxx/logs/2026_03_25.log
```

The file rotates lazily on the first write after midnight.

## Loading The Logger

Logging is initialized by calling `Load()`.

`Load()`:

1. registers the `log.level` default if it is not already declared
2. resolves the shared log level from config
3. creates the console and file handlers
4. replaces the package-local writer state
5. sets the global `slog` default logger
6. supplies `*log.Level` into the container
7. ensures a shutdown hook exists to close the writer

Application code normally does not call `Load()` directly. The framework boot
sequence does that for you.

Tests may call `Load()` again after `config.Load()` to rebuild logging state,
matching the mental model used by `framework/config`.

## Configuration

The package uses one configuration key:

| Key | Env Var | Default | Description |
|-----|---------|---------|-------------|
| `log.level` | `LOG_LEVEL` | `"INFO"` | Shared minimum level for both console and file sinks |

There is no separate console-only or file-only level.

## Log Levels

Use levels consistently:

- `DEBUG` for detailed trace and diagnosis
- `INFO` for normal operations and business events
- `WARN` for degraded but recoverable behavior
- `ERROR` for failed work that needs attention

Invalid log levels cause `Load()` to panic during boot.

## Writing Logs

The package does not wrap `slog`. Log through the Go standard library
directly.

Basic example:

```go
slog.Info("user created", "user_id", user.ID, "email", user.Email)
slog.Error("payment failed", "error", err, "amount", amount)
```

Prefer static messages with structured attributes.

Do this:

```go
slog.Info("order created", "user_id", userID, "order_id", orderID)
```

Not this:

```go
slog.Info(fmt.Sprintf("user %s created order %s", userID, orderID))
```

## Context-Aware Logging

The logger automatically injects request-scoped metadata from the context.

Supported context fields:

- `request_id`
- `user_id`

Store them with:

```go
ctx = log.WithRequestID(ctx, requestID)
ctx = log.WithUserID(ctx, userID)
```

Then log with the context-aware `slog` methods:

```go
slog.InfoContext(ctx, "creating order", "product_id", productID)
slog.ErrorContext(ctx, "order creation failed", "error", err)
```

If you log without context, those fields are simply omitted.

## Runtime Level Changes

The shared log level is available through the container as `*log.Level`.

Example:

```go
lv := container.MustMake[*log.Level]()
lv.Set(slog.LevelDebug)
```

This updates both console and file logging because the package uses one shared
level variable.

## Shutdown And Flushing

The framework registers a shutdown hook that calls `Close()` during graceful
shutdown.

`Close()`:

- flushes any buffered file output
- closes the active file handle
- clears the package-local writer state

`Flush()` is also available when you need buffered file output on disk before
shutdown, especially in tests.

## Testing

Tests should follow the same lifecycle model as the application:

```go
t.Setenv("DATA_DIR", t.TempDir())
container.Reset()
config.Load()
log.Load()
defer log.Close()
```

This keeps logging aligned with the current config and avoids hidden test-only
reset pathways.

## Notes

- always use an isolated `DATA_DIR` when running locally or in tests
- the package is write-only by design
- if you later need log browsing or retention tooling, build it outside
  `framework/log`
