# Configuration

## Introduction

The `framework/config` package provides process-wide configuration for a
Sunkern application.

It is responsible for:

- loading configuration from `DATA_DIR/config.json`
- reading environment variable overrides
- registering framework and module defaults
- coercing raw values into typed Go values
- validating config during boot
- exposing the declared config surface for CLI or diagnostics

Unlike larger frameworks that split configuration across many files, Sunkern
keeps configuration intentionally simple: one runtime config file, one
environment override layer, and explicit defaults registered by the framework
and modules.

## Configuration Sources

Each configuration key resolves in the following order:

1. Environment variables
2. `{DATA_DIR}/config.json`
3. Registered defaults

The package uses dot notation for keys:

- `app.env`
- `http.addr`
- `db.busy_timeout`
- `jwt.secret`

Environment variables are derived by uppercasing the key and replacing dots
with underscores:

- `app.env` becomes `APP_ENV`
- `http.addr` becomes `HTTP_ADDR`
- `jwt.secret` becomes `JWT_SECRET`

You may convert a key to its environment variable name with
`EnvName`.

## Loading Configuration

Configuration is initialized by calling `Load()`.

During `Load()`:

1. `DATA_DIR` is resolved from the environment, falling back to
   `~/.standalone`
2. the data directory is created if it does not already exist
3. `{DATA_DIR}/config.json` is read if present
4. nested JSON is flattened into dot-notation keys
5. `data_dir` is registered as a default key

Example:

```go
config.Load()

dataDir := config.DataDir()
```

If `config.json` is missing, the package continues normally. If the file exists
but contains invalid JSON, `Load()` panics immediately.

## Environment Configuration

Environment variables are the highest-priority configuration layer. They are
typically used for deployment-specific values, secrets, and one-off overrides.

For example, this key:

```json
{
  "http": {
    "addr": ":19110"
  }
}
```

may be overridden by setting:

```bash
HTTP_ADDR=:8080
```

If both `config.json` and the environment define a key, the environment value
wins.

## Defining Configuration Values

Framework packages and feature modules define their configuration surface by
registering defaults during boot.

Use `SetDefault` for a single key:

```go
config.SetDefault("http.addr", ":19110")
```

Use `SetDefaults` for a group of flat keys:

```go
config.SetDefaults(config.Values{
	"http.read_timeout":  "15s",
	"http.write_timeout": "15s",
	"http.idle_timeout":  "60s",
})
```

`config.Values` is an alias for `map[string]any`. It exists purely to keep
call sites and examples compact.

Defaults serve two purposes:

- they provide fallback values when no higher-priority source exists
- they declare the application's known configuration surface

That second point matters for introspection helpers such as `All()`, `Keys()`,
and `Sub()`.

## Accessing Configuration Values

Use `Get[T]` when a value must exist and must be valid:

```go
addr := config.Get[string]("http.addr")
```

`Get[T]` panics if:

- the key does not exist in any layer
- the raw value cannot be coerced to `T`

Use `GetOr[T]` when you want a fallback:

```go
readTimeout := config.GetOr[time.Duration]("http.read_timeout", 15*time.Second)
```

Use `Has` to check for existence without coercion:

```go
if config.Has("jwt.secret") {
	// ...
}
```

Use `Ensure` to fail fast when required values are missing:

```go
config.Ensure("jwt.secret", "resend.api_token")
```

## Supported Types

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

If a raw config value cannot be converted to the requested target type, `Get`
panics and `GetOr` returns its fallback value.

## Integer Values

Integer coercion is intentionally strict.

Accepted values:

- integer strings such as `"19110"`
- JSON whole numbers such as `19110`

Rejected values:

- fractional strings such as `"19110.5"`
- fractional JSON numbers such as `19110.5`

The package does not silently truncate configuration values. If a value is
meant to be an integer, it must be provided as an integer.

## Duration Values

`time.Duration` values must be expressed as duration strings when they come
from environment variables or `config.json`.

Accepted values:

- `"15s"`
- `"5m"`
- `"1h30m"`

Rejected values:

- `"30"`
- `30`
- `30.5`

Typed Go defaults of type `time.Duration` are still supported:

```go
config.SetDefault("job.interval", 30*time.Second)
```

This keeps runtime configuration explicit and readable while still allowing
framework code to register strongly typed defaults.

## Validation

Configuration rules are registered with `AddRule` and checked by `Validate()`.

Built-in rules include:

- `Required`
- `NotEmpty`
- `Positive`
- `NonNegative`
- `OneOf(...)`
- `Min(...)`
- `Max(...)`
- `Range(...)`
- `MinLen(...)`
- `MaxLen(...)`

Example:

```go
config.AddRule("log.level", config.OneOf("DEBUG", "INFO", "WARN", "ERROR"))
config.AddRule("jwt.secret", config.Required, config.MinLen(32))
```

If validation fails, `Validate()` panics with a summary of every failure.

## Configuration Lifecycle

The expected lifecycle is:

1. call `Load()`
2. register defaults with `SetDefault` or `SetDefaults`
3. register validation rules with `AddRule`
4. call `Validate()`
5. read values with `Get` and `GetOr`

After validation succeeds, the package is frozen. Any later call to
`SetDefault`, `SetDefaults`, or `AddRule` panics.

This keeps the config surface stable after boot and prevents modules from
quietly changing configuration shape during runtime.

## Accessing Declared Configuration

The package includes a small introspection layer for CLI commands and
diagnostics.

`All()` returns the effective value of every known key.

`Keys()` returns the sorted list of known keys.

`Sub(prefix)` returns the effective values for a namespace with the prefix removed.

For example, if the application knows these keys:

- `db.host`
- `db.port`
- `db.trace`

then:

```go
db := config.Sub("db")
```

returns a map with:

- `host`
- `port`
- `trace`

`Sub("db.")` is also accepted.

## Known Keys

`All()`, `Keys()`, and `Sub()` only operate on known keys.

A key becomes known when it appears in either:

- registered defaults
- `config.json`

This means env-only undeclared keys are still readable through `Get()` and
`Has()`, but they are not listed by `All()`, `Keys()`, or `Sub()`.

This behavior is intentional. The declared configuration surface should come
from the application itself, not from whatever extra environment variables may
exist in the host process.

## Practical Example

```go
config.Load()

config.SetDefaults(config.Values{
	"http.addr":         ":19110",
	"http.read_timeout": "15s",
})

config.AddRule("http.addr", config.NotEmpty)

config.Validate()

addr := config.Get[string]("http.addr")
timeout := config.GetOr[time.Duration]("http.read_timeout", 15*time.Second)
```

In this example:

- `HTTP_ADDR` overrides `config.json`
- `config.json` overrides the registered defaults
- invalid values fail during `Get` or `Validate`

## Notes

- `DATA_DIR` should be set explicitly when running Sunkern apps locally or in
  tests
- `config.json` uses nested JSON, but the package resolves it as flat
  dot-notation keys
- environment values arrive as strings and are coerced on read
- `GetOr` falls back on both missing values and coercion failures
