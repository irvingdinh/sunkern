# Configuration

All configuration for a Sunkern application flows through the `framework/config` package. It provides a three-layer resolution system — environment variables, a JSON configuration file, and registered defaults — so that every config value has a clear, predictable source of truth.

The package uses flat dot-notation keys (e.g., `db.host`, `http.port`) that map directly to `UPPER_SNAKE_CASE` environment variables. This makes it trivial for container platforms like Fly.io, Railway, and Cloud Run to override any setting without touching files.

```go
import "sunkern.local/framework/config"

func main() {
    config.Load()
    port := config.Get[int]("http.port")
}
```

## Configuration Files

Configuration is read from a single JSON file at `{DATA_DIR}/config.json`. The file is optional — if it does not exist, the package silently continues with environment variables and defaults.

Nested JSON structures are automatically flattened into dot-notation keys during `Load()`:

```json
{
  "http": {
    "port": 19110,
    "read_timeout": "15s"
  },
  "db": {
    "busy_timeout": 5000,
    "synchronous": "NORMAL"
  }
}
```

After loading, these become flat keys:

| Key | Value |
|-----|-------|
| `http.port` | `19110` |
| `http.read_timeout` | `"15s"` |
| `db.busy_timeout` | `5000` |
| `db.synchronous` | `"NORMAL"` |

> **Note:** The config file must contain valid JSON. If the file exists but is malformed, `Load()` panics immediately.

## Environment Variables

Every dot-notation key maps to an `UPPER_SNAKE_CASE` environment variable. The conversion is simple: uppercase everything and replace dots with underscores.

| Config Key | Environment Variable |
|------------|---------------------|
| `http.port` | `HTTP_PORT` |
| `db.host` | `DB_HOST` |
| `jwt.secret` | `JWT_SECRET` |
| `email.from_address` | `EMAIL_FROM_ADDRESS` |
| `log.level` | `LOG_LEVEL` |
| `data_dir` | `DATA_DIR` |

Use `config.EnvName(key)` to get the environment variable name programmatically:

```go
envVar := config.EnvName("jwt.secret") // "JWT_SECRET"
```

Sunkern uses **flat, unprefixed** environment variable names (e.g., `PORT`, not `SUNKERN_PORT`). These are standalone apps — one per container — so namespace collisions are not a concern.

## Resolution Order

When you request a config value, the package checks three layers in order. The first match wins:

```
1. Environment variable   ← highest priority
2. config.json value
3. Registered default     ← lowest priority
```

This design lets you ship sensible defaults in code, override them with a config file for specific deployments, and override everything with environment variables for container platforms.

```go
// Module registers a default during boot:
config.SetDefault("http.port", 19110)

// config.json may contain: {"http": {"port": 8080}}

// Environment may have: HTTP_PORT=3000

// Result of config.Get[int]("http.port"):
//   HTTP_PORT set     → 3000      (env wins)
//   No env var        → 8080      (config.json wins)
//   No env, no file   → 19110     (default wins)
```

> **Note:** Environment variables always arrive as strings. The package coerces them to the requested type automatically — see [Type Coercion](#type-coercion).

## Retrieving Configuration Values

### Required Values

Use `Get[T](key)` for configuration that must exist. It panics with a descriptive message (including the environment variable name) if the key is missing from all three layers or if the value cannot be coerced to type `T`.

```go
port := config.Get[int]("http.port")
secret := config.Get[string]("jwt.secret")
```

Use this for boot-time configuration that the application cannot run without.

### Optional Values

Use `GetOr[T](key, fallback)` for configuration with safe defaults. It returns the fallback value if the key is missing or coercion fails. It never panics.

```go
timeout := config.GetOr("http.read_timeout", 15*time.Second)
debug := config.GetOr("app.debug", false)
workers := config.GetOr("queue.workers", 4)
```

### Existence Checks

Use `Has(key)` to check whether a key can be resolved from any layer without attempting type coercion. Zero values (`""`, `0`, `false`) count as existing.

```go
if config.Has("resend.api_token") {
    // Email sending is available
}
```

### Data Directory

`DataDir()` is a convenience shorthand for `Get[string]("data_dir")`. It returns the absolute path to the application's data directory.

```go
logsDir := filepath.Join(config.DataDir(), "logs")
dbPath := filepath.Join(config.DataDir(), "database.sqlite")
```

The data directory is resolved during `Load()`:
1. If the `DATA_DIR` environment variable is set, use it
2. Otherwise, default to `~/.standalone`
3. The directory is created automatically (with `0o755` permissions) if it does not exist

> **Warning:** When running multiple Sunkern instances on the same machine (or in tests), always set `DATA_DIR` to an isolated directory. The default `~/.standalone` is shared and will cause data collisions.

## Setting Defaults

Register default values during the module registration phase — before `Validate()` runs.

### Single Key

```go
config.SetDefault("http.port", 19110)
```

### Multiple Keys

Use `SetDefaults` with `config.Values` (an alias for `map[string]any`) to register related keys together:

```go
func (m *Module) Register() {
    config.SetDefaults(config.Values{
        "http.port":         19110,
        "http.read_timeout": "15s",
        "http.idle_timeout": "60s",
    })
}
```

When the same key is registered multiple times, the last call wins. This is how a service module can override a framework-level default.

> **Warning:** Calling `SetDefault` or `SetDefaults` after the config is frozen (after `Validate()`) causes a panic.

## Validation

### Adding Rules

Use `AddRule(key, ...Rule)` to register validation functions during the registration phase. Multiple `AddRule` calls for the same key accumulate — all rules must pass.

```go
func (m *Module) Register() {
    config.SetDefault("http.port", 19110)
    config.AddRule("http.port", config.Range(1, 65535))

    config.AddRule("jwt.secret", config.NotEmpty, config.MinLen(32))
    config.AddRule("log.level", config.OneOf("DEBUG", "INFO", "WARN", "ERROR"))
}
```

### Running Validation

The framework calls `Validate()` after all modules have registered their defaults and rules, but before any module's `Boot()` phase. It runs every registered rule and panics with a summary of all violations:

```
config validation failed:
  - HTTP_PORT: must be in [1, 65535], got 0
  - JWT_SECRET: must not be empty
```

After `Validate()` completes (even with no violations), the config is frozen.

### Built-in Rules

| Rule | Description |
|------|-------------|
| `Required` | Key must exist in at least one layer. Zero values (`""`, `0`, `false`) are valid. |
| `NotEmpty` | Key must exist AND its string value must be non-empty. |
| `Positive` | Integer value must be strictly greater than zero. |
| `NonNegative` | Integer value must be zero or greater. |
| `OneOf(values...)` | String value must be one of the listed values (case-sensitive). |
| `Min(n)` | Integer value must be >= n. |
| `Max(n)` | Integer value must be <= n. |
| `Range(min, max)` | Integer value must be in [min, max] inclusive. |
| `MinLen(n)` | String value must have at least n characters. |
| `MaxLen(n)` | String value must have at most n characters. |

All built-in rules (except `Required` and `NotEmpty`) pass silently if the key does not exist. This lets you validate a value only when it is provided.

### Custom Rules

A `Rule` is a function with the signature `func(key string, value any, exists bool) error`. Return `nil` to pass, an error to fail:

```go
config.AddRule("app.mode", func(key string, value any, exists bool) error {
    if !exists {
        return nil // optional key
    }
    s, ok := value.(string)
    if !ok {
        return fmt.Errorf("must be a string")
    }
    if s != "development" && s != "production" {
        return fmt.Errorf("must be 'development' or 'production', got '%s'", s)
    }
    return nil
})
```

### Ensuring Required Keys

`Ensure(keys...)` is an imperative existence check. It panics immediately with a summary listing all missing keys by their environment variable names:

```go
config.Ensure("jwt.secret", "resend.api_token")
// Panics: "missing required config: JWT_SECRET, RESEND_API_TOKEN"
```

Unlike `Required` (which is declarative and runs during `Validate`), `Ensure` runs at the call site. Use it when you need to assert key existence at a specific point in your boot sequence.

## Type Coercion

The package automatically coerces values to the requested type when you call `Get[T]` or `GetOr[T]`. This is especially useful for environment variables, which always arrive as strings.

| Target Type | Coerces From | Notes |
|-------------|-------------|-------|
| `string` | any | Always succeeds via `fmt.Sprintf` |
| `bool` | string, int, float64 | Strings: `true`/`1`/`t`/`yes`/`on`, `false`/`0`/`f`/`no`/`off` (case-insensitive) |
| `int` | float64, int64, string, bool | Floats with fractional part are rejected |
| `int32`, `int64` | Same as `int` | Overflow is checked and rejected |
| `uint`, `uint8`–`uint64` | int, float64, string, bool | Negative values are rejected |
| `float64` | int, int64, string, bool | |
| `time.Time` | string | Tries in order: RFC3339, RFC3339Nano, `2006-01-02`, `2006-01-02 15:04:05` |
| `time.Duration` | string | Go duration format: `"15s"`, `"1m30s"`, `"168h"` |
| `[]int` | []any, comma-separated string | Each element coerced individually |
| `[]string` | []any, comma-separated string | Each element stringified |
| `map[string]any` | Direct type match only | No automatic conversion |
| `map[string]string` | map[string]any | Values stringified |
| `map[string][]string` | map[string]any | Single values wrapped in slice |

> **Note:** Comma-separated strings (e.g., `ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173`) are split into slices automatically when you request `[]string` or `[]int`.

## Freeze

After `Validate()` completes, the config is frozen. This prevents accidental registration of new defaults or rules during the runtime phase.

```go
config.IsFrozen() // true after Validate()
config.SetDefault("new.key", "value") // PANIC: config is frozen
config.AddRule("new.key", config.Required) // PANIC: config is frozen
```

You can call `Freeze()` manually for earlier lockdown, but this is rarely needed — `Validate()` handles it automatically.

Query functions (`Get`, `GetOr`, `Has`, `All`, `Keys`, `Sub`) are unaffected by freeze and work normally throughout the application lifecycle.

## Introspection

Three functions expose the known configuration for diagnostics and CLI tooling.

### All

`All()` returns a snapshot of all known keys with their resolved values (applying the full resolution order):

```go
snapshot := config.All()
// {"http.port": 19110, "db.host": "localhost", ...}
```

### Keys

`Keys()` returns a sorted list of all known key names:

```go
keys := config.Keys()
// ["db.host", "db.port", "http.port", ...]
```

### Sub

`Sub(prefix)` returns all keys under a prefix with the prefix stripped and resolution applied:

```go
dbConfig := config.Sub("db")
// {"host": "localhost", "port": 5432, "busy_timeout": 5000}
```

Both `Sub("db")` and `Sub("db.")` work — trailing dots are handled automatically.

> **Note:** All three functions only include keys that were registered via `SetDefault` or present in `config.json`. Keys that exist only as environment variables are not visible in introspection results.

## Lifecycle Summary

```
1. config.Load()
   ├── Resolves DATA_DIR (env var or ~/.standalone)
   ├── Creates data directory if missing
   ├── Reads and flattens config.json (if exists)
   └── Sets "data_dir" as a default key

2. Module Registration Phase
   ├── Modules call SetDefault / SetDefaults
   └── Modules call AddRule

3. config.Validate()
   ├── Runs all registered rules
   ├── Panics with summary if any rule fails
   └── Freezes config (no more SetDefault / AddRule)

4. Runtime
   ├── Get[T] / GetOr[T] / Has / DataDir — read config values
   └── All / Keys / Sub — introspection
```

Calling `Load()` again resets all state (clears defaults, file values, and rules). This is useful in tests to start with a clean slate.
