package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type state struct {
	mu       sync.RWMutex
	values   map[string]any // from JSON file (flattened)
	defaults map[string]any // from SetDefault calls
}

var global = state{
	values:   make(map[string]any),
	defaults: make(map[string]any),
}

// Load resolves the data directory, loads {data_dir}/config.json, and
// initializes the global config state. It panics if the config file exists
// but contains malformed JSON. A missing config file is silently ignored.
//
// The data directory is resolved from the DATA_DIR environment variable,
// falling back to ~/.standalone. The resolved value is stored as a default
// so callers can retrieve it via Get[string]("data_dir").
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
	global.values = flat
	global.defaults = defaults
	global.mu.Unlock()
}

// SetDefault registers a default value for a key. Called by modules during
// their registration phase. Defaults have lower priority than config file
// values and environment variables.
func SetDefault(key string, value any) {
	global.mu.Lock()
	global.defaults[key] = value
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

// Get returns the value for key, coerced to type T. It panics if the key is
// not found in any layer, or if the value cannot be coerced to T.
func Get[T any](key string) T {
	raw, ok := resolve(key)
	if !ok {
		panic(fmt.Sprintf("config: key not found: %q", key))
	}
	var zero T
	val, ok := coerce[T](raw)
	if !ok {
		panic(fmt.Sprintf("config: cannot coerce %q to %T", key, zero))
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

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

// resolve returns the value for a key, checking all three layers.
// Resolution order: env var → config file → registered defaults.
func resolve(key string) (any, bool) {
	envKey := keyToEnvVar(key)
	if envVal, ok := os.LookupEnv(envKey); ok {
		return envVal, true
	}

	global.mu.RLock()
	defer global.mu.RUnlock()

	if val, ok := global.values[key]; ok {
		return val, true
	}

	if val, ok := global.defaults[key]; ok {
		return val, true
	}

	return nil, false
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
