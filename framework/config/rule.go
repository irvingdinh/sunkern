package config

import (
	"fmt"
	"sort"
	"strings"
)

// AddRule registers one or more validation rules for a config key. Rules are
// checked when Validate is called. Multiple AddRule calls for the same key
// accumulate — all rules must pass.
//
// Call AddRule during the Register phase (alongside SetDefault). Rules
// registered after Validate has run will not be checked.
//
//	config.SetDefault("http.port", 19110)
//	config.AddRule("http.port", config.Range(1, 65535))
//
//	config.AddRule("log.level", config.OneOf("DEBUG", "INFO", "WARN", "ERROR"))
func AddRule(key string, rules ...Rule) {
	global.mu.Lock()
	if global.frozen {
		global.mu.Unlock()
		panic(fmt.Sprintf("config: AddRule(%q) called after config is frozen", key))
	}
	global.rules[key] = append(global.rules[key], rules...)
	global.mu.Unlock()
}

// Validate runs all registered rules and panics with a summary of every
// violation. The panic message lists each failing key with its environment
// variable name and the specific error.
//
// Called by the app framework after all modules have registered defaults
// and rules, and after framework services have initialized — but before
// any module's Boot phase.
func Validate() {
	// Snapshot rules under read lock, then release before calling resolve
	// (which acquires its own read lock). This avoids potential deadlock
	// if a writer is waiting between the two acquisitions.
	global.mu.RLock()
	snapshot := make(map[string][]Rule, len(global.rules))
	for k, v := range global.rules {
		snapshot[k] = v
	}
	global.mu.RUnlock()

	if len(snapshot) == 0 {
		return
	}

	var errs []string
	for key, rules := range snapshot {
		val, exists := resolve(key)
		for _, rule := range rules {
			if err := rule(key, val, exists); err != nil {
				errs = append(errs, fmt.Sprintf("  %s (%s): %s", key, keyToEnvVar(key), err))
			}
		}
	}

	if len(errs) > 0 {
		sort.Strings(errs)
		panic(fmt.Sprintf("config: validation failed:\n%s", strings.Join(errs, "\n")))
	}

	// Freeze config after successful validation. No further SetDefault,
	// SetDefaults, MarkSensitive, or AddRule calls are allowed.
	Freeze()
}

// ---------------------------------------------------------------------------
// Built-in rules
// ---------------------------------------------------------------------------

// Required ensures the key exists in at least one config layer (environment
// variable, config file, or registered default). Zero values (empty string,
// 0, false) are considered valid — use NotEmpty for stricter checks.
var Required Rule = func(_ string, _ any, exists bool) error {
	if !exists {
		return fmt.Errorf("required but not set")
	}
	return nil
}

// NotEmpty ensures the key exists and its string representation is not empty.
// Useful for secrets and credentials that must have a non-trivial value.
var NotEmpty Rule = func(_ string, value any, exists bool) error {
	if !exists {
		return fmt.Errorf("required but not set")
	}
	s, ok := coerce[string](value)
	if !ok || s == "" {
		return fmt.Errorf("must not be empty")
	}
	return nil
}

// Positive ensures a numeric value is strictly greater than zero.
var Positive Rule = func(_ string, value any, exists bool) error {
	if !exists {
		return nil
	}
	n, ok := coerce[int](value)
	if !ok {
		return fmt.Errorf("must be a positive integer")
	}
	if n <= 0 {
		return fmt.Errorf("must be positive, got %d", n)
	}
	return nil
}

// NonNegative ensures a numeric value is zero or greater.
var NonNegative Rule = func(_ string, value any, exists bool) error {
	if !exists {
		return nil
	}
	n, ok := coerce[int](value)
	if !ok {
		return fmt.Errorf("must be a non-negative integer")
	}
	if n < 0 {
		return fmt.Errorf("must be non-negative, got %d", n)
	}
	return nil
}

// OneOf ensures the string value is one of the allowed values. The check is
// case-sensitive. If the key does not exist, the rule passes — combine with
// Required to enforce both existence and membership.
//
//	config.AddRule("log.level", config.Required, config.OneOf("DEBUG", "INFO", "WARN", "ERROR"))
func OneOf(allowed ...string) Rule {
	return func(_ string, value any, exists bool) error {
		if !exists {
			return nil
		}
		s, ok := coerce[string](value)
		if !ok {
			return fmt.Errorf("must be a string")
		}
		for _, a := range allowed {
			if s == a {
				return nil
			}
		}
		return fmt.Errorf("must be one of [%s], got %q", strings.Join(allowed, ", "), s)
	}
}

// Min ensures a numeric value is at least min (inclusive).
func Min(min int) Rule {
	return func(_ string, value any, exists bool) error {
		if !exists {
			return nil
		}
		n, ok := coerce[int](value)
		if !ok {
			return fmt.Errorf("must be an integer")
		}
		if n < min {
			return fmt.Errorf("must be >= %d, got %d", min, n)
		}
		return nil
	}
}

// Max ensures a numeric value is at most max (inclusive).
func Max(max int) Rule {
	return func(_ string, value any, exists bool) error {
		if !exists {
			return nil
		}
		n, ok := coerce[int](value)
		if !ok {
			return fmt.Errorf("must be an integer")
		}
		if n > max {
			return fmt.Errorf("must be <= %d, got %d", max, n)
		}
		return nil
	}
}

// Range ensures a numeric value is within [min, max] inclusive.
//
//	config.AddRule("http.port", config.Range(1, 65535))
func Range(min, max int) Rule {
	return func(_ string, value any, exists bool) error {
		if !exists {
			return nil
		}
		n, ok := coerce[int](value)
		if !ok {
			return fmt.Errorf("must be an integer")
		}
		if n < min || n > max {
			return fmt.Errorf("must be in [%d, %d], got %d", min, max, n)
		}
		return nil
	}
}

// MinLen ensures a string value has at least n characters.
//
//	config.AddRule("jwt.secret", config.NotEmpty, config.MinLen(32))
func MinLen(n int) Rule {
	return func(_ string, value any, exists bool) error {
		if !exists {
			return nil
		}
		s, ok := coerce[string](value)
		if !ok {
			return fmt.Errorf("must be a string")
		}
		if len(s) < n {
			return fmt.Errorf("must be at least %d characters, got %d", n, len(s))
		}
		return nil
	}
}

// MaxLen ensures a string value has at most n characters.
func MaxLen(n int) Rule {
	return func(_ string, value any, exists bool) error {
		if !exists {
			return nil
		}
		s, ok := coerce[string](value)
		if !ok {
			return fmt.Errorf("must be a string")
		}
		if len(s) > n {
			return fmt.Errorf("must be at most %d characters, got %d", n, len(s))
		}
		return nil
	}
}
