---
title: Framework Configuration Package
impact: HIGH
impactDescription: incorrect config usage causes panics at boot or silent fallback behavior at runtime
tags: config, configuration, environment, defaults, validation, coercion
---

## Framework Configuration Package

**Package:** `sunkern.local/framework/config`

The config package provides a three-layer, boot-time configuration system.
Configuration is loaded once during startup, then read throughout the app. The
framework owns `config.Load()`; modules should define defaults, add rules, and
read typed values.

### API Surface

```go
// Register one default value. Call during Register.
config.SetDefault(key string, value any)

// Register many defaults at once from a flat map.
config.SetDefaults(config.Values{
    "http.addr": ":19110",
})

// Read a required value. Panics on missing key or invalid coercion.
config.Get[T any](key string) T

// Read an optional value. Returns fallback on missing key or invalid coercion.
config.GetOr[T any](key string, defaultVal T) T

// Check that required keys exist in any layer.
config.Ensure(keys ...string)

// Register validation rules during Register.
config.AddRule(key string, rules ...config.Rule)

// Validate all rules and freeze the config package.
config.Validate()
```

### Resolution Order

Values resolve in this order:

1. environment variable
2. `{DATA_DIR}/config.json`
3. registered default

The first match wins.

### Key Naming

Config keys use dot notation and map to upper snake case environment variables:

| Config Key | Environment Variable |
|------------|----------------------|
| `http.addr` | `HTTP_ADDR` |
| `db.busy_timeout` | `DB_BUSY_TIMEOUT` |
| `jwt.secret` | `JWT_SECRET` |
| `data_dir` | `DATA_DIR` |

Use `config.EnvName("jwt.secret")` when code needs the env var name.

### Correct Usage Patterns

**Register defaults during Register:**

```go
func (m *PaymentModule) Register() {
    config.SetDefaults(config.Values{
        "payment.timeout":     "30s",
        "payment.max_retries": 3,
        "payment.currency":    "USD",
    })

    config.AddRule("payment.max_retries", config.NonNegative)
}
```

**Read required config with `Get`:**

```go
func (m *PaymentModule) Boot(ctx context.Context) error {
    apiKey := config.Get[string]("payment.api_key")
    _ = apiKey
    return nil
}
```

**Read optional config with `GetOr`:**

```go
func (m *PaymentModule) Boot(ctx context.Context) error {
    timeout := config.GetOr[time.Duration]("payment.timeout", 30*time.Second)
    retries := config.GetOr[int]("payment.max_retries", 3)
    _, _ = timeout, retries
    return nil
}
```

**Fail fast on required keys:**

```go
func (m *PaymentModule) Boot(ctx context.Context) error {
    config.Ensure("payment.api_key", "payment.webhook_secret")
    return nil
}
```

### Incorrect Usage Patterns

**Do not call `Load()` from modules:**

```go
func (m *PaymentModule) Register() {
    config.Load() // WRONG: resets all config state
}
```

**Do not register defaults in Boot:**

```go
func (m *PaymentModule) Boot(ctx context.Context) error {
    config.SetDefault("payment.timeout", "30s") // WRONG: too late
    return nil
}
```

**Do not use `Get` for optional config:**

```go
port := config.Get[int]("optional.debug.port") // WRONG: may panic
```

Use:

```go
port := config.GetOr[int]("optional.debug.port", 0)
```

**Do not parse durations manually after reading strings:**

```go
timeoutStr := config.Get[string]("payment.timeout") // WRONG
timeout, _ := time.ParseDuration(timeoutStr)
```

Use:

```go
timeout := config.Get[time.Duration]("payment.timeout")
```

### Type Coercion Rules

The package supports typed reads for:

- `string`
- `bool`
- `int`, `int32`, `int64`
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- `float64`
- `time.Time`
- `time.Duration`
- `[]int`
- `[]string`
- `map[string]any`
- `map[string]string`
- `map[string][]string`

Important behavior:

- environment variables always arrive as strings
- `Get` panics on invalid coercion
- `GetOr` falls back on invalid coercion
- integer targets reject fractional JSON numbers
- `time.Duration` accepts duration strings such as `"15s"` or `"1h30m"`
- `time.Duration` does not accept bare numeric strings or JSON numbers

If you want a duration, store it as a duration string in config and env. Do
not rely on implicit nanosecond parsing.

### Validation And Freeze

`AddRule` registers boot-time validation. `Validate()` checks all rules and then
freezes the package.

After validation, these calls are no longer allowed:

- `SetDefault`
- `SetDefaults`
- `AddRule`

Any of them will panic after freeze.

### Introspection Helpers

The package keeps a small introspection layer:

- `All()` returns the effective value of every known key
- `Keys()` returns the sorted list of known keys
- `Sub("db")` returns keys under a namespace with the prefix removed
- `Sub("db.")` is also accepted

Known-key boundary:

- defaults and `config.json` define known keys
- env overrides are reflected for known keys
- env-only undeclared keys are readable via `Get` and `Has`
- env-only undeclared keys are not listed by `All`, `Keys`, or `Sub`

This is intentional. Generic tooling should reflect the application's declared
config surface, not arbitrary host environment variables.

### Removed / Unsupported Concepts

Do not design against config metadata or admin-snapshot APIs. The current
package does not expose:

- config descriptions
- sensitive-key masking metadata
- exported config entry snapshots
- config diffs

If code or docs mention `Describe`, `MarkSensitive`, `Export`, or `Diff`,
assume that guidance is stale.

### Data Directory Isolation

Always run Sunkern apps with an isolated `DATA_DIR`, especially in tests and
local development:

```bash
export DATA_DIR=/tmp/sunkern_data_$(date +%s)
```

Never rely on `~/.standalone` when multiple local projects may run on the same
machine.
