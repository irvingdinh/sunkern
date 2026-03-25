package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Rule validates a resolved config value. It receives the key name, the
// resolved value (or nil if missing), and whether the key exists in any
// layer. Return a non-nil error to indicate a validation failure.
type Rule func(key string, value any, exists bool) error

// Source indicates which layer a config value was resolved from.
type Source string

const (
	// SourceEnv means the value was resolved from an environment variable.
	SourceEnv Source = "env"
	// SourceFile means the value was loaded from config.json.
	SourceFile Source = "file"
	// SourceDefault means the value was registered via SetDefault.
	SourceDefault Source = "default"
)

// Values is a convenience alias for flat config maps, mainly for SetDefaults
// calls and examples.
type Values = map[string]any

type state struct {
	mu           sync.RWMutex
	frozen       bool
	values       map[string]any            // from JSON file (flattened)
	defaults     map[string]any            // from SetDefault calls
	rules        map[string][]Rule         // from AddRule calls
}

var global = state{
	values:   make(map[string]any),
	defaults: make(map[string]any),
	rules:    make(map[string][]Rule),
}

// Load resolves the data directory, creates it if needed, loads
// {data_dir}/config.json, and initializes the global config state. It panics
// if the config file exists but contains malformed JSON. A missing config file
// is silently ignored.
//
// The data directory is resolved from the DATA_DIR environment variable,
// falling back to ~/.standalone. The resolved value is stored as a default
// so callers can retrieve it via Get[string]("data_dir") or DataDir().
//
// The data directory is created with 0o755 permissions if it does not already
// exist. Downstream packages (sqlite, log) can rely on it being present.
//
// Calling Load again resets all state (both values and defaults). This is
// useful in tests to get a clean config between test cases.
func Load() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			panic(fmt.Sprintf("config: resolving home directory: %v", err))
		}
		dataDir = filepath.Join(home, ".standalone")
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		panic(fmt.Sprintf("config: creating data directory %q: %v", dataDir, err))
	}

	path := filepath.Join(dataDir, "config.json")
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		panic(fmt.Sprintf("config: reading config file: %v", err))
	}

	var flat map[string]any
	if data != nil {
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			panic(fmt.Sprintf("config: parsing config file: %v", err))
		}
		flat = make(map[string]any)
		flatten("", raw, flat)
	}

	defaults := make(map[string]any)
	defaults["data_dir"] = dataDir

	global.mu.Lock()
	global.frozen = false
	global.values = flat
	global.defaults = defaults
	global.rules = make(map[string][]Rule)
	global.mu.Unlock()
}

// SetDefault registers a default value for a key. Called by modules during
// their registration phase. Defaults have the lowest priority — config file
// values and environment variables take precedence.
//
// When multiple modules call SetDefault for the same key, the last call wins.
// Panics if the config has been frozen (after Validate).
func SetDefault(key string, value any) {
	global.mu.Lock()
	if global.frozen {
		global.mu.Unlock()
		panic(fmt.Sprintf("config: SetDefault(%q) called after config is frozen", key))
	}
	global.defaults[key] = value
	global.mu.Unlock()
}

// SetDefaults registers multiple default values at once from a flat map.
// Equivalent to calling SetDefault for each entry. Useful for modules that
// need to register many defaults during their registration phase.
//
//	config.SetDefaults(Values{
//	    "http.port":         19110,
//	    "http.read_timeout": "15s",
//	    "http.idle_timeout": "60s",
//	})
func SetDefaults(m Values) {
	global.mu.Lock()
	if global.frozen {
		global.mu.Unlock()
		panic("config: SetDefaults called after config is frozen")
	}
	for k, v := range m {
		global.defaults[k] = v
	}
	global.mu.Unlock()
}

// Ensure panics if any of the given keys cannot be resolved from any layer
// (environment variable, config file, or registered default). The panic
// message lists all missing keys using their environment variable names.
//
// Ensure checks existence only — zero values (empty string, 0, false) are
// considered valid as long as the key exists in at least one layer.
func Ensure(keys ...string) {
	var missing []string
	for _, key := range keys {
		if _, ok := resolve(key); !ok {
			missing = append(missing, keyToEnvVar(key))
		}
	}
	if len(missing) > 0 {
		panic(fmt.Sprintf("config: required values not set: %s", strings.Join(missing, ", ")))
	}
}

// Has reports whether a key can be resolved from any layer (environment
// variable, config file, or registered default). It does not attempt type
// coercion — it only checks for existence.
func Has(key string) bool {
	_, ok := resolve(key)
	return ok
}

// Get returns the value for key, coerced to type T. It panics if the key is
// not found in any layer, or if the value cannot be coerced to T. The panic
// message includes the corresponding environment variable name as a hint.
func Get[T any](key string) T {
	raw, ok := resolve(key)
	if !ok {
		panic(fmt.Sprintf("config: key %q not found (set %s env var or add to config.json)", key, keyToEnvVar(key)))
	}
	var zero T
	val, ok := coerce[T](raw)
	if !ok {
		panic(fmt.Sprintf("config: cannot coerce %q to %T (raw value: %v)", key, zero, raw))
	}
	return val
}

// GetOr returns the value for key, coerced to type T. If the key is not
// found in any layer, or if coercion fails, it returns defaultVal.
func GetOr[T any](key string, defaultVal T) T {
	raw, ok := resolve(key)
	if !ok {
		return defaultVal
	}
	val, ok := coerce[T](raw)
	if !ok {
		return defaultVal
	}
	return val
}

// DataDir returns the resolved data directory path. This is a convenience
// shorthand for Get[string]("data_dir") — the data directory is central to
// the standalone architecture and accessed frequently.
func DataDir() string {
	return Get[string]("data_dir")
}

// EnvName returns the environment variable name that corresponds to the
// given dot-notation config key. Useful for error messages and CLI output.
//
//	config.EnvName("jwt.secret") // "JWT_SECRET"
func EnvName(key string) string {
	return keyToEnvVar(key)
}

// All returns a snapshot of every known config key with its resolved value.
// For each key found in defaults or the config file, the resolution order is
// applied (env var > config file > default). Keys that exist only as
// environment variables and were never referenced by SetDefault or the config
// file are not included — use Get or Has for those.
//
// The returned map is a copy and safe to modify.
func All() map[string]any {
	global.mu.RLock()
	defer global.mu.RUnlock()

	result := make(map[string]any)

	// Start with defaults.
	for k, v := range global.defaults {
		result[k] = v
	}

	// Overlay config file values.
	for k, v := range global.values {
		result[k] = v
	}

	// Check env var overrides for every known key.
	for k := range result {
		envKey := keyToEnvVar(k)
		if envVal, ok := os.LookupEnv(envKey); ok {
			result[k] = envVal
		}
	}

	return result
}

// Keys returns a sorted list of all known config keys (from defaults and
// the config file). Keys that exist only as environment variables and were
// never referenced by SetDefault or the config file are not included.
func Keys() []string {
	global.mu.RLock()
	defer global.mu.RUnlock()

	seen := make(map[string]struct{})
	for k := range global.defaults {
		seen[k] = struct{}{}
	}
	for k := range global.values {
		seen[k] = struct{}{}
	}

	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Sub returns all resolved key-value pairs under the given prefix, with the
// prefix stripped from the keys. For example, if the config contains
// "db.host", "db.port", and "http.port", calling Sub("db") returns
// {"host": ..., "port": ...}. The returned map is a copy.
//
// Returns nil if no keys match the prefix.
func Sub(prefix string) map[string]any {
	prefix = strings.TrimSuffix(prefix, ".")
	dotPrefix := prefix + "."

	global.mu.RLock()
	defer global.mu.RUnlock()

	result := make(map[string]any)

	// Collect keys from defaults and file values under prefix.
	for k, v := range global.defaults {
		if strings.HasPrefix(k, dotPrefix) {
			result[strings.TrimPrefix(k, dotPrefix)] = v
		}
	}
	for k, v := range global.values {
		if strings.HasPrefix(k, dotPrefix) {
			result[strings.TrimPrefix(k, dotPrefix)] = v
		}
	}

	// Check env var overrides for discovered keys.
	for sub := range result {
		fullKey := prefix + "." + sub
		envKey := keyToEnvVar(fullKey)
		if envVal, ok := os.LookupEnv(envKey); ok {
			result[sub] = envVal
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// Freeze prevents further SetDefault, SetDefaults, and AddRule calls. Called
// automatically at the end of Validate. Can also be called manually to freeze
// config earlier.
//
// Freeze is idempotent — calling it multiple times is safe.
func Freeze() {
	global.mu.Lock()
	global.frozen = true
	global.mu.Unlock()
}

// IsFrozen reports whether the config has been frozen.
func IsFrozen() bool {
	global.mu.RLock()
	frozen := global.frozen
	global.mu.RUnlock()
	return frozen
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

// resolve returns the value for a key, checking all three layers.
// Resolution order: env var → config file → registered defaults.
func resolve(key string) (any, bool) {
	val, _, ok := resolveWithSource(key)
	return val, ok
}

// resolveWithSource returns the value for a key along with which layer it
// was resolved from. Resolution order: env var → config file → defaults.
func resolveWithSource(key string) (any, Source, bool) {
	envKey := keyToEnvVar(key)
	if envVal, ok := os.LookupEnv(envKey); ok {
		return envVal, SourceEnv, true
	}

	global.mu.RLock()
	defer global.mu.RUnlock()

	if val, ok := global.values[key]; ok {
		return val, SourceFile, true
	}

	if val, ok := global.defaults[key]; ok {
		return val, SourceDefault, true
	}

	return nil, "", false
}

// keyToEnvVar converts a dot-notation key to an environment variable name.
// "email.from_address" → "EMAIL_FROM_ADDRESS"
func keyToEnvVar(key string) string {
	return strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
}

// flatten recursively walks a nested map and produces a flat map with
// dot-separated keys.
func flatten(prefix string, m map[string]any, out map[string]any) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			flatten(key, val, out)
		default:
			out[key] = val
		}
	}
}
