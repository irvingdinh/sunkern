---
title: Framework Configuration Package
impact: HIGH
impactDescription: incorrect config usage causes panics at boot or silent fallback behavior at runtime
tags: config, configuration, environment, defaults, validation, coercion
---

## Framework Configuration Package

**Location:** `framework/config/`

The `framework/config` package provides three-layer configuration resolution with flat dot-notation keys. Priority order: environment variables > `config.json` > registered defaults. The package freezes after validation, making the config surface immutable at runtime.

---

### Resolution Order

Every config lookup follows a strict priority chain. Environment variables always win.

```
1. Environment variable  (highest priority)
2. config.json value
3. SetDefault value      (lowest priority)
```

**Incorrect (assuming config.json overrides env vars):**

```go
// Developer sets port=8080 in config.json, expects it to take effect.
// But PORT=3000 is set in the environment — the env var wins silently.
// The developer is confused why port 3000 is used.
port := config.Get[int]("port") // returns 3000, not 8080
```

**Correct (understanding that env vars always take precedence):**

```go
// Env vars override everything. This is intentional — it lets container
// platforms (Fly.io, Railway) control config without touching files.
port := config.Get[int]("port")
// PORT=3000 in env → 3000
// No env var, port: 8080 in config.json → 8080
// No env var, no file → falls back to SetDefault value
```

---

### Setting Defaults

Register defaults during the module's `Register` phase only. Use `SetDefaults` with `config.Values{...}` to group related keys.

**Incorrect (registering defaults at runtime or after validation):**

```go
func (m *Module) Boot() error {
    // PANIC: config is frozen after Validate() runs before Boot()
    config.SetDefault("mymod.timeout", "30s")
    return nil
}
```

**Incorrect (registering defaults one by one when they belong together):**

```go
func (m *Module) Register() {
    config.SetDefault("email.from", "noreply@example.com")
    config.SetDefault("email.provider", "resend")
    config.SetDefault("email.enabled", false)
}
```

**Correct (using SetDefaults in Register for grouped flat keys):**

```go
func (m *Module) Register() {
    config.SetDefaults(config.Values{
        "email.from":     "noreply@example.com",
        "email.provider": "resend",
        "email.enabled":  false,
    })
}
```

---

### Required Config Access

`Get[T](key)` panics if the key is missing from all three layers or if coercion to type `T` fails. Use it for config that must exist at boot.

**Incorrect (using Get[T] for optional config that may not be set):**

```go
func (m *Module) Boot() error {
    // PANIC if RESEND_API_TOKEN is not set — but this is optional config.
    // Email sending only works when the token is provided.
    token := config.Get[string]("resend.api_token")
    return nil
}
```

**Correct (using Get[T] for truly required boot-time config):**

```go
func (m *Module) Boot() error {
    // Port must exist — the server cannot start without it.
    // If missing, the panic message includes the env var name: "PORT".
    port := config.Get[int]("http.port")
    server.Listen(port)
    return nil
}
```

---

### Optional Config Access

`GetOr[T](key, fallback)` returns the fallback value when the key is missing or coercion fails. It never panics.

**Incorrect (using Get[T] where GetOr[T] is appropriate):**

```go
// This panics if HTTP_READ_TIMEOUT is not set anywhere.
timeout := config.Get[time.Duration]("http.read_timeout")
```

**Correct (using GetOr[T] with a sensible fallback):**

```go
// Falls back to 15s if not configured — a safe default.
timeout := config.GetOr("http.read_timeout", 15*time.Second)
```

---

### Validation and Freeze

`AddRule` registers validation functions during the registration phase. `Validate` runs all rules and freezes the config — no more `SetDefault` or `AddRule` calls are allowed after this point.

**Incorrect (calling AddRule after Validate):**

```go
func (m *Module) Boot() error {
    // PANIC: config is frozen — AddRule is not allowed after Validate().
    config.AddRule("mymod.workers", config.Positive)
    return nil
}
```

**Incorrect (skipping Validate and relying on Get[T] to catch problems):**

```go
// Without Validate, invalid config is only discovered when first accessed.
// By then the app may be partially booted, leaving it in an inconsistent state.
```

**Correct (AddRule during registration, Validate runs in the boot sequence):**

```go
func (m *Module) Register() {
    config.SetDefaults(config.Values{
        "http.port":         19110,
        "http.read_timeout": "15s",
    })
    config.AddRule("http.port", config.Range(1, 65535))
    config.AddRule("http.read_timeout", config.Required)
}

// The framework calls config.Validate() after all modules register.
// All rule violations are reported at once, then config freezes.
```

**Built-in rules:**

| Rule | Description |
|------|-------------|
| `Required` | Key must exist (zero values like `""`, `0`, `false` are valid) |
| `NotEmpty` | Key must exist AND string value must be non-empty |
| `Positive` | Integer must be > 0 |
| `NonNegative` | Integer must be >= 0 |
| `OneOf(values...)` | String must match one of the listed values |
| `Min(n)` | Integer must be >= n |
| `Max(n)` | Integer must be <= n |
| `Range(min, max)` | Integer must be in [min, max] |
| `MinLen(n)` | String length must be >= n |
| `MaxLen(n)` | String length must be <= n |

---

### Ensuring Required Keys

`Ensure(keys...)` validates that all given keys can be resolved. It panics with a summary listing missing keys by their environment variable names. Use it for imperative existence checks at a specific point — not for value validation.

**Incorrect (using Ensure for value constraints):**

```go
// Ensure only checks existence, not value. Use AddRule for constraints.
config.Ensure("http.port") // passes even if port is 0 or negative
```

**Correct (using Ensure to assert critical keys exist before proceeding):**

```go
func (m *Module) Boot() error {
    // Panics with: "missing required config: JWT_SECRET, RESEND_API_TOKEN"
    config.Ensure("jwt.secret", "resend.api_token")
    return nil
}
```

---

### Key Naming Convention

Config keys use lowercase dot-notation. The package maps them to `UPPER_SNAKE_CASE` environment variables automatically.

| Config Key | Environment Variable |
|------------|---------------------|
| `http.port` | `HTTP_PORT` |
| `db.host` | `DB_HOST` |
| `jwt.secret` | `JWT_SECRET` |
| `email.from_address` | `EMAIL_FROM_ADDRESS` |
| `log.level` | `LOG_LEVEL` |

Use `config.EnvName(key)` to get the env var name programmatically.

**Incorrect (non-standard key formats):**

```go
config.SetDefault("db_host", "localhost")     // underscores instead of dots
config.SetDefault("dbHost", "localhost")      // camelCase
config.SetDefault("DB.HOST", "localhost")     // uppercase
```

**Correct (lowercase dot-separated keys):**

```go
config.SetDefault("db.host", "localhost")
config.SetDefault("db.port", 5432)
config.SetDefault("db.busy_timeout", 5000)
```

---

### Type Coercion

Environment variables arrive as strings. The package coerces them to the requested type on read. Do not manually parse strings — let `Get[T]` and `GetOr[T]` handle coercion.

**Supported target types:** `string`, `bool`, `int`, `int32`, `int64`, `uint`, `uint8`–`uint64`, `float64`, `time.Time`, `time.Duration`, `[]int`, `[]string`, `map[string]any`, `map[string]string`, `map[string][]string`.

**Incorrect (manually parsing env strings):**

```go
portStr := os.Getenv("HTTP_PORT")
port, err := strconv.Atoi(portStr)
if err != nil {
    port = 19110
}
```

**Correct (letting the package coerce):**

```go
port := config.GetOr("http.port", 19110)
// Handles: int from JSON, string "19110" from env, default 19110
```

**Coercion notes:**
- **bool** from string: accepts `true/1/t/yes/on` and `false/0/f/no/off` (case-insensitive)
- **int** from float64: rejected if fractional part is non-zero
- **uint** from int/float64: rejected if negative
- **time.Time** from string: tries RFC3339, RFC3339Nano, `2006-01-02`, `2006-01-02 15:04:05`
- **time.Duration** from string: uses `time.ParseDuration` (e.g., `"15s"`, `"1m30s"`, `"168h"`)
- **[]int** / **[]string** from string: splits by comma, trims whitespace

---

### Introspection

`All()`, `Keys()`, and `Sub(prefix)` expose known configuration for CLI tooling and diagnostics. Keys that exist only as environment variables and were never registered via `SetDefault` or present in `config.json` are excluded.

**Incorrect (expecting All() to discover env-only keys):**

```go
// MY_CUSTOM_VAR is set in the environment but never registered as a default.
all := config.All()
val := all["my.custom_var"] // nil — not in the snapshot
```

**Correct (using introspection for declared keys):**

```go
config.SetDefaults(config.Values{
    "db.host": "localhost",
    "db.port": 5432,
})

dbConfig := config.Sub("db")
// Returns: {"host": "localhost", "port": 5432}
// With env overrides applied if DB_HOST or DB_PORT are set.

allKeys := config.Keys()
// Returns sorted: ["db.host", "db.port", ...]
```

---

### No Metadata Layer

Do not add metadata APIs like `Describe`, `MarkSensitive`, `Export`, or `Diff` to the config package. The package is intentionally minimal: get, set default, validate, freeze.

**Incorrect (inventing metadata APIs):**

```go
// These APIs do not exist and should not be added.
config.Describe("jwt.secret", "Secret key for JWT signing")
config.MarkSensitive("jwt.secret")
config.Export() // dump all config with descriptions and sensitivity flags
```

**Correct (keeping the config surface minimal):**

```go
// Config keys are self-documenting through their names and validation rules.
config.SetDefault("jwt.secret", "")
config.AddRule("jwt.secret", config.NotEmpty, config.MinLen(32))
// Documentation lives in README.md and skill rules, not in the config API.
```

---

### Data Directory

Always set `DATA_DIR` to an isolated temporary directory in tests and when running multiple Sunkern instances. Never rely on the default `~/.standalone` — it causes data collisions.

**Incorrect (using default data directory in tests):**

```go
func TestSomething(t *testing.T) {
    config.Load() // uses ~/.standalone — shared with other tests and dev server
    // Test data bleeds into production data directory
}
```

**Correct (isolated temp directory per test):**

```go
func TestSomething(t *testing.T) {
    t.Setenv("DATA_DIR", t.TempDir())
    config.Load()
    // Each test gets its own clean data directory
}
```
