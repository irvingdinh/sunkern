---
title: Framework Configuration Package
impact: HIGH
impactDescription: incorrect config usage causes panics at boot or silent misconfiguration at runtime
tags: config, configuration, environment, defaults, coercion
---

## Framework Configuration Package

**Package:** `sunkern.local/framework/config`

The config package provides a three-layer, boot-time configuration system. Config is static — set before or at application start, never changed at runtime. The framework calls `config.Load()` automatically during app startup; modules only interact with defaults and reads.

---

### API Reference

```go
// Set a default value (lowest priority). Call in module Register phase.
config.SetDefault(key string, value any)

// Read a value, panic if missing or coercion fails. Use for required config.
config.Get[T any](key string) T

// Read a value, return fallback if missing or coercion fails. Never panics.
config.GetOr[T any](key string, defaultVal T) T

// Validate that all keys exist in any layer. Panics listing missing env var names.
config.Ensure(keys ...string)

// Load is called by the framework — never call it from module code.
config.Load()
```

---

### Resolution Order

Values are resolved highest-priority-first. The first match wins:

1. **Environment variable** — `os.LookupEnv(KEY_NAME)` (always checked first)
2. **Config file** — `{DATA_DIR}/config.json` (flattened dot-notation)
3. **Registered default** — from `config.SetDefault()` calls (lowest priority)

---

### Key Naming Convention

Config keys use **dot-notation**. They map to **UPPER_SNAKE_CASE** environment variables:

| Config Key | Environment Variable |
|------------|---------------------|
| `http.addr` | `HTTP_ADDR` |
| `log.level` | `LOG_LEVEL` |
| `db.busy_timeout` | `DB_BUSY_TIMEOUT` |
| `data_dir` | `DATA_DIR` |
| `jwt.secret` | `JWT_SECRET` |

Transformation: uppercase, replace `.` with `_`.

---

### Correct Usage Patterns

**Setting defaults in a module's Register phase:**

```go
func (m *PaymentModule) Register() {
    config.SetDefault("payment.timeout", "30s")
    config.SetDefault("payment.max_retries", 3)
    config.SetDefault("payment.currency", "USD")
}
```

**Reading required config (panics if missing):**

```go
func (m *PaymentModule) Boot(ctx context.Context) error {
    secret := config.Get[string]("payment.api_key")
    // Use secret — if PAYMENT_API_KEY env var is not set and no default
    // or config.json entry exists, this panics immediately.
}
```

**Reading optional config with fallback:**

```go
func (m *PaymentModule) Boot(ctx context.Context) error {
    timeout := config.GetOr[time.Duration]("payment.timeout", 30*time.Second)
    retries := config.GetOr[int]("payment.max_retries", 3)
}
```

**Validating required keys at boot:**

```go
func (m *PaymentModule) Boot(ctx context.Context) error {
    // Panics with: "config: required values not set: PAYMENT_API_KEY, PAYMENT_WEBHOOK_SECRET"
    config.Ensure("payment.api_key", "payment.webhook_secret")
}
```

---

### Incorrect Usage Patterns

**Calling SetDefault outside of Register phase:**

```go
// WRONG: SetDefault in Boot is too late — other modules may have already
// read this key during their Boot phase.
func (m *PaymentModule) Boot(ctx context.Context) error {
    config.SetDefault("payment.timeout", "30s") // too late
}
```

**Calling config.Load() from module code:**

```go
// WRONG: Load() resets ALL state (values and defaults). The framework
// calls it once during app startup. Calling it again wipes everything.
func (m *PaymentModule) Register() {
    config.Load() // destroys all previously registered defaults
}
```

**Using Get[T] for optional config:**

```go
// WRONG: panics if the key doesn't exist.
port := config.Get[int]("optional.debug.port")

// CORRECT: use GetOr with a sensible fallback.
port := config.GetOr[int]("optional.debug.port", 0)
```

**Relying on string type for everything:**

```go
// WRONG: reading as string then parsing manually.
timeoutStr := config.Get[string]("payment.timeout")
timeout, _ := time.ParseDuration(timeoutStr)

// CORRECT: let the coercion system handle it.
timeout := config.Get[time.Duration]("payment.timeout")
```

---

### Supported Type Coercions

The generic type parameter in `Get[T]` and `GetOr[T]` supports:

| Type | Coerces from | Notes |
|------|-------------|-------|
| `string` | anything | Uses `fmt.Sprintf("%v", raw)` |
| `bool` | string, int, float64 | Strings: "true", "1", "yes", "on", "t" (case-insensitive) and inverses |
| `int` | int, int64, float64, string, bool | Overflow-checked against `math.MaxInt` |
| `int32` | int, int32, int64, float64, string, bool | Overflow-checked |
| `int64` | int, int32, int64, float64, string, bool | Overflow-checked |
| `uint` | uint, int, int64, float64, string, bool | Rejects negative values |
| `uint8` | uint8, int, int64, float64, string, bool | Range [0, 255] |
| `uint16` | uint16, int, int64, float64, string, bool | Range [0, 65535] |
| `uint32` | uint32, int, int64, float64, string, bool | Range checked |
| `uint64` | uint64, uint, int, int64, float64, string, bool | Rejects negative |
| `float64` | float64, int, int64, string, bool | Overflow-checked |
| `time.Time` | time.Time, string | Tries: RFC3339, RFC3339Nano, "2006-01-02", "2006-01-02 15:04:05" |
| `time.Duration` | duration, string, int, int64, float64 | String: Go format ("15m", "2h30m") or bare nanoseconds |
| `[]int` | []int, []any, string | String: comma-separated ("1, 2, 3") |
| `[]string` | []string, []any, string | String: comma-separated ("a, b, c") |
| `map[string]any` | map[string]any | Direct cast only |
| `map[string]string` | map[string]string, map[string]any | Values coerced to string |
| `map[string][]string` | map[string][]string, map[string]any | Values coerced to string slices |

Environment variables are always strings. The coercion layer handles `"8080"` (string) to `int(8080)` automatically.

---

### Config File Format

The config file is **JSON** at `{DATA_DIR}/config.json`. Nested objects are flattened to dot-notation:

```json
{
  "http": {
    "addr": ":8080"
  },
  "log": {
    "level": "DEBUG"
  },
  "payment": {
    "timeout": "30s",
    "max_retries": 3
  }
}
```

This flattens to keys: `http.addr`, `log.level`, `payment.timeout`, `payment.max_retries`.

Missing file is silently ignored (config works with env vars and defaults alone). Malformed JSON causes a panic at boot.

---

### Data Directory Isolation

**For testing and development**, always set `DATA_DIR` to an isolated directory:

```bash
export DATA_DIR=/tmp/sunkern_data_$(date +%s)
```

**Never use the default** `~/.standalone` when multiple Sunkern projects may run concurrently — it causes data collisions.

The `data_dir` key is automatically available after `config.Load()`:

```go
dataDir := config.Get[string]("data_dir")
```

---

### Thread Safety

- `Get[T]` and `GetOr[T]` are concurrent-read safe (RWMutex read lock)
- `SetDefault` acquires an exclusive write lock
- `Load()` acquires an exclusive write lock and resets all state
- Safe to call `Get`/`GetOr` from any goroutine after boot

---

### Error Behavior Summary

| Function | Key missing | Coercion fails | Returns |
|----------|------------|----------------|---------|
| `Get[T]` | panic | panic | T |
| `GetOr[T]` | returns fallback | returns fallback | T |
| `Ensure` | panic (lists all missing) | n/a | void |
| `SetDefault` | n/a | n/a | void |
