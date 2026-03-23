package config

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// loadEmpty calls Load with DATA_DIR pointing to an empty temp dir.
func loadEmpty(t *testing.T) {
	t.Helper()
	t.Setenv("DATA_DIR", t.TempDir())
	Load()
}

// loadWithJSON writes a config.json to a temp dir and calls Load.
func loadWithJSON(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing config.json: %v", err)
	}
	t.Setenv("DATA_DIR", dir)
	Load()
}

// mustPanic asserts that fn panics and the panic message contains substr.
func mustPanic(t *testing.T, substr string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, substr) {
			t.Errorf("panic message = %q, want substring %q", msg, substr)
		}
	}()
	fn()
}

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

func TestLoadResetsState(t *testing.T) {
	loadEmpty(t)
	SetDefault("keep.me", "first")
	if got := GetOr[string]("keep.me", ""); got != "first" {
		t.Fatalf("before reset: got %q, want %q", got, "first")
	}

	loadEmpty(t) // reset
	got := GetOr[string]("keep.me", "gone")
	if got != "gone" {
		t.Errorf("after reset: got %q, want %q (state should be cleared)", got, "gone")
	}
}

func TestLoadMissingFile(t *testing.T) {
	loadEmpty(t) // should not panic
}

func TestLoadMalformedFile(t *testing.T) {
	mustPanic(t, "parsing config file", func() {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{invalid`), 0o644)
		t.Setenv("DATA_DIR", dir)
		Load()
	})
}

func TestLoadResolvesDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	Load()
	if got := Get[string]("data_dir"); got != dir {
		t.Errorf("data_dir = %q, want %q", got, dir)
	}
}

// ---------------------------------------------------------------------------
// SetDefault
// ---------------------------------------------------------------------------

func TestSetDefault(t *testing.T) {
	loadEmpty(t)
	SetDefault("app.name", "sunkern")
	if got := GetOr[string]("app.name", ""); got != "sunkern" {
		t.Errorf("got %q, want %q", got, "sunkern")
	}
}

func TestSetDefaultLastWins(t *testing.T) {
	loadEmpty(t)
	SetDefault("port", 8080)
	SetDefault("port", 9090)
	if got := GetOr[int]("port", 0); got != 9090 {
		t.Errorf("got %d, want 9090 (last SetDefault wins)", got)
	}
}

// ---------------------------------------------------------------------------
// Resolution order
// ---------------------------------------------------------------------------

func TestResolutionOrder(t *testing.T) {
	loadWithJSON(t, `{
		"port": 8080,
		"log": {"level": "debug"},
		"jwt": {"secret": "file-secret"}
	}`)
	SetDefault("port", 19110)
	SetDefault("log.level", "info")
	SetDefault("jwt.secret", "")
	t.Setenv("PORT", "443")
	t.Setenv("JWT_SECRET", "env-secret")

	// port: default 19110 → file 8080 → env 443
	if got := Get[int]("port"); got != 443 {
		t.Errorf("port = %d, want 443", got)
	}
	// log.level: default "info" → file "debug" → no env → "debug"
	if got := Get[string]("log.level"); got != "debug" {
		t.Errorf("log.level = %q, want %q", got, "debug")
	}
	// jwt.secret: default "" → file "file-secret" → env "env-secret"
	if got := Get[string]("jwt.secret"); got != "env-secret" {
		t.Errorf("jwt.secret = %q, want %q", got, "env-secret")
	}
	// cache.max_entries: no file, no env, no default → fallback
	if got := GetOr[int]("cache.max_entries", 5000); got != 5000 {
		t.Errorf("cache.max_entries = %d, want 5000", got)
	}
}

func TestEnvOverridesJSON(t *testing.T) {
	loadWithJSON(t, `{"port": 8080}`)
	t.Setenv("PORT", "9999")
	if got := Get[int]("port"); got != 9999 {
		t.Errorf("got %d, want 9999", got)
	}
}

func TestJSONOverridesDefault(t *testing.T) {
	loadWithJSON(t, `{"cache": {"max_entries": 20000}}`)
	SetDefault("cache.max_entries", 10000)
	if got := Get[int]("cache.max_entries"); got != 20000 {
		t.Errorf("got %d, want 20000", got)
	}
}

// ---------------------------------------------------------------------------
// Get / GetOr
// ---------------------------------------------------------------------------

func TestGetPanicsOnMissing(t *testing.T) {
	loadEmpty(t)
	mustPanic(t, "key not found", func() {
		Get[string]("nonexistent.key")
	})
}

func TestGetOrReturnsFallback(t *testing.T) {
	loadEmpty(t)
	if got := GetOr[string]("missing", "fallback"); got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestGetCoercionFailure(t *testing.T) {
	loadEmpty(t)
	t.Setenv("PORT", "abc")

	// GetOr falls back on coercion failure
	if got := GetOr[int]("port", 19110); got != 19110 {
		t.Errorf("got %d, want 19110 (coercion failure fallback)", got)
	}

	// Get panics on coercion failure
	mustPanic(t, "cannot coerce", func() {
		Get[int]("port")
	})
}

func TestGetUnsupportedType(t *testing.T) {
	loadEmpty(t)
	SetDefault("key", "value")

	// GetOr returns default for unsupported type
	type custom struct{}
	if got := GetOr[custom]("key", custom{}); got != (custom{}) {
		t.Errorf("expected zero custom struct")
	}
}

// ---------------------------------------------------------------------------
// String coercion
// ---------------------------------------------------------------------------

func TestGetString(t *testing.T) {
	loadWithJSON(t, `{"name": "sunkern"}`)
	SetDefault("env", "development")
	t.Setenv("HOST", "localhost")

	if got := Get[string]("name"); got != "sunkern" {
		t.Errorf("name = %q, want %q", got, "sunkern")
	}
	if got := Get[string]("env"); got != "development" {
		t.Errorf("env = %q, want %q", got, "development")
	}
	if got := Get[string]("host"); got != "localhost" {
		t.Errorf("host = %q, want %q", got, "localhost")
	}

	// Non-string coercion
	SetDefault("port", 8080)
	if got := Get[string]("port"); got != "8080" {
		t.Errorf("port as string = %q, want %q", got, "8080")
	}
}

// ---------------------------------------------------------------------------
// Int coercion
// ---------------------------------------------------------------------------

func TestGetInt(t *testing.T) {
	loadWithJSON(t, `{"port": 8080}`)
	SetDefault("workers", 4)
	t.Setenv("TIMEOUT", "30")

	if got := Get[int]("port"); got != 8080 {
		t.Errorf("port = %d, want 8080", got)
	}
	if got := Get[int]("workers"); got != 4 {
		t.Errorf("workers = %d, want 4", got)
	}
	if got := Get[int]("timeout"); got != 30 {
		t.Errorf("timeout = %d, want 30", got)
	}
}

func TestGetInt32(t *testing.T) {
	loadWithJSON(t, `{"code": 200}`)
	SetDefault("retry", int32(3))
	t.Setenv("STATUS", "404")

	if got := Get[int32]("code"); got != 200 {
		t.Errorf("code = %d, want 200", got)
	}
	if got := Get[int32]("retry"); got != 3 {
		t.Errorf("retry = %d, want 3", got)
	}
	if got := Get[int32]("status"); got != 404 {
		t.Errorf("status = %d, want 404", got)
	}
	// Overflow
	SetDefault("big", int64(math.MaxInt32+1))
	if got := GetOr[int32]("big", -1); got != -1 {
		t.Errorf("big = %d, want -1 (overflow fallback)", got)
	}
}

func TestGetInt64(t *testing.T) {
	loadWithJSON(t, `{"big": 9999999999}`)
	SetDefault("id", int64(42))
	t.Setenv("OFFSET", "1000000000000")

	if got := Get[int64]("big"); got != 9999999999 {
		t.Errorf("big = %d, want 9999999999", got)
	}
	if got := Get[int64]("id"); got != 42 {
		t.Errorf("id = %d, want 42", got)
	}
	if got := Get[int64]("offset"); got != 1000000000000 {
		t.Errorf("offset = %d, want 1000000000000", got)
	}
}

func TestGetUint(t *testing.T) {
	loadEmpty(t)
	SetDefault("workers", 8)
	t.Setenv("LIMIT", "100")

	if got := Get[uint]("workers"); got != 8 {
		t.Errorf("workers = %d, want 8", got)
	}
	if got := Get[uint]("limit"); got != 100 {
		t.Errorf("limit = %d, want 100", got)
	}
	// Negative returns fallback
	SetDefault("neg", -1)
	if got := GetOr[uint]("neg", 99); got != 99 {
		t.Errorf("neg = %d, want 99 (negative fallback)", got)
	}
}

func TestGetUint8(t *testing.T) {
	loadEmpty(t)
	SetDefault("level", 5)
	t.Setenv("PRIORITY", "255")

	if got := Get[uint8]("level"); got != 5 {
		t.Errorf("level = %d, want 5", got)
	}
	if got := Get[uint8]("priority"); got != 255 {
		t.Errorf("priority = %d, want 255", got)
	}
	// Overflow
	SetDefault("big", 256)
	if got := GetOr[uint8]("big", 1); got != 1 {
		t.Errorf("big = %d, want 1 (overflow fallback)", got)
	}
}

func TestGetUint16(t *testing.T) {
	loadEmpty(t)
	t.Setenv("PORT", "8080")

	if got := Get[uint16]("port"); got != 8080 {
		t.Errorf("port = %d, want 8080", got)
	}
	// Overflow
	SetDefault("big", 70000)
	if got := GetOr[uint16]("big", 1); got != 1 {
		t.Errorf("big = %d, want 1 (overflow fallback)", got)
	}
}

func TestGetUint32(t *testing.T) {
	loadWithJSON(t, `{"count": 100000}`)
	t.Setenv("MAX", "4294967295")

	if got := Get[uint32]("count"); got != 100000 {
		t.Errorf("count = %d, want 100000", got)
	}
	if got := Get[uint32]("max"); got != math.MaxUint32 {
		t.Errorf("max = %d, want MaxUint32", got)
	}
}

func TestGetUint64(t *testing.T) {
	loadEmpty(t)
	t.Setenv("SNOWFLAKE", "18446744073709551615")

	if got := Get[uint64]("snowflake"); got != math.MaxUint64 {
		t.Errorf("snowflake = %d, want MaxUint64", got)
	}
	// Negative returns fallback
	SetDefault("neg", -1)
	if got := GetOr[uint64]("neg", 42); got != 42 {
		t.Errorf("neg = %d, want 42 (negative fallback)", got)
	}
}

// ---------------------------------------------------------------------------
// Bool coercion
// ---------------------------------------------------------------------------

func TestGetBool(t *testing.T) {
	cases := []struct {
		envVal string
		want   bool
	}{
		{"true", true}, {"false", false},
		{"1", true}, {"0", false},
		{"TRUE", true}, {"FALSE", false},
		{"yes", true}, {"no", false},
		{"on", true}, {"off", false},
		{"t", true}, {"f", false},
	}
	for _, tc := range cases {
		t.Run(tc.envVal, func(t *testing.T) {
			loadEmpty(t)
			t.Setenv("FLAG", tc.envVal)
			if got := Get[bool]("flag"); got != tc.want {
				t.Errorf("GetBool with env %q = %v, want %v", tc.envVal, got, tc.want)
			}
		})
	}

	// From JSON
	loadWithJSON(t, `{"db": {"wal_mode": true}}`)
	if got := Get[bool]("db.wal_mode"); !got {
		t.Errorf("db.wal_mode = %v, want true", got)
	}
}

// ---------------------------------------------------------------------------
// Float64 coercion
// ---------------------------------------------------------------------------

func TestGetFloat64(t *testing.T) {
	loadWithJSON(t, `{"threshold": 0.75}`)
	SetDefault("rate", 1.5)
	t.Setenv("FACTOR", "2.5")

	if got := Get[float64]("threshold"); got != 0.75 {
		t.Errorf("threshold = %f, want 0.75", got)
	}
	if got := Get[float64]("rate"); got != 1.5 {
		t.Errorf("rate = %f, want 1.5", got)
	}
	if got := Get[float64]("factor"); got != 2.5 {
		t.Errorf("factor = %f, want 2.5", got)
	}
}

// ---------------------------------------------------------------------------
// Time coercion
// ---------------------------------------------------------------------------

func TestGetTime(t *testing.T) {
	loadEmpty(t)
	zero := time.Time{}

	SetDefault("launch", "2025-06-15T10:00:00Z")
	want := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	if got := Get[time.Time]("launch"); !got.Equal(want) {
		t.Errorf("launch = %v, want %v", got, want)
	}

	SetDefault("birthday", "1990-01-15")
	wantDate := time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC)
	if got := Get[time.Time]("birthday"); !got.Equal(wantDate) {
		t.Errorf("birthday = %v, want %v", got, wantDate)
	}

	t.Setenv("DEADLINE", "2026-12-31T23:59:59Z")
	wantDeadline := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	if got := Get[time.Time]("deadline"); !got.Equal(wantDeadline) {
		t.Errorf("deadline = %v, want %v", got, wantDeadline)
	}

	// Unparseable returns fallback
	SetDefault("bad", "not-a-date")
	sentinel := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := GetOr[time.Time]("bad", sentinel); !got.Equal(sentinel) {
		t.Errorf("bad = %v, want sentinel %v", got, sentinel)
	}

	// Missing returns fallback
	if got := GetOr[time.Time]("missing", zero); !got.Equal(zero) {
		t.Errorf("missing = %v, want zero", got)
	}
}

// ---------------------------------------------------------------------------
// Duration coercion
// ---------------------------------------------------------------------------

func TestGetDuration(t *testing.T) {
	loadEmpty(t)

	// String durations
	SetDefault("auth.access_ttl", "15m")
	if got := Get[time.Duration]("auth.access_ttl"); got != 15*time.Minute {
		t.Errorf("access_ttl = %v, want 15m", got)
	}

	t.Setenv("AUTH_REFRESH_TTL", "168h")
	if got := Get[time.Duration]("auth.refresh_ttl"); got != 168*time.Hour {
		t.Errorf("refresh_ttl = %v, want 168h", got)
	}

	// Bare int → time.Duration(v), caller multiplies
	SetDefault("timeout", 30)
	raw := Get[time.Duration]("timeout")
	if raw*time.Second != 30*time.Second {
		t.Errorf("timeout * Second = %v, want 30s", raw*time.Second)
	}
}

// ---------------------------------------------------------------------------
// Slice coercion
// ---------------------------------------------------------------------------

func TestGetIntSlice(t *testing.T) {
	loadEmpty(t)

	// From SetDefault
	SetDefault("ports", []int{80, 443, 8080})
	got := Get[[]int]("ports")
	if len(got) != 3 || got[0] != 80 || got[1] != 443 || got[2] != 8080 {
		t.Errorf("ports = %v, want [80 443 8080]", got)
	}

	// From env (comma-separated)
	t.Setenv("RETRIES", "1, 2, 3")
	got = Get[[]int]("retries")
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("retries = %v, want [1 2 3]", got)
	}

	// From JSON ([]any with float64)
	loadWithJSON(t, `{"ids": [10, 20, 30]}`)
	got = Get[[]int]("ids")
	if len(got) != 3 || got[0] != 10 || got[1] != 20 || got[2] != 30 {
		t.Errorf("ids = %v, want [10 20 30]", got)
	}

	// Invalid env returns fallback
	t.Setenv("BAD_LIST", "a,b,c")
	got = GetOr[[]int]("bad_list", []int{99})
	if len(got) != 1 || got[0] != 99 {
		t.Errorf("bad_list = %v, want [99]", got)
	}
}

func TestGetStringSlice(t *testing.T) {
	loadEmpty(t)

	// From SetDefault
	SetDefault("origins", []string{"http://localhost", "https://example.com"})
	got := Get[[]string]("origins")
	if len(got) != 2 || got[0] != "http://localhost" {
		t.Errorf("origins = %v", got)
	}

	// From env (comma-separated)
	t.Setenv("TAGS", "go, backend, api")
	got = Get[[]string]("tags")
	if len(got) != 3 || got[0] != "go" || got[1] != "backend" || got[2] != "api" {
		t.Errorf("tags = %v, want [go backend api]", got)
	}

	// From JSON ([]any)
	loadWithJSON(t, `{"cors": {"methods": ["GET", "POST"]}}`)
	got = Get[[]string]("cors.methods")
	if len(got) != 2 || got[0] != "GET" || got[1] != "POST" {
		t.Errorf("cors.methods = %v, want [GET POST]", got)
	}
}

// ---------------------------------------------------------------------------
// Map coercion
// ---------------------------------------------------------------------------

func TestGetStringMap(t *testing.T) {
	loadEmpty(t)
	SetDefault("db", map[string]any{"host": "localhost", "port": 5432})

	got := Get[map[string]any]("db")
	if got["host"] != "localhost" || got["port"] != 5432 {
		t.Errorf("db = %v", got)
	}

	// Missing returns fallback
	def := map[string]any{"a": 1}
	if got := GetOr[map[string]any]("missing", def); got["a"] != 1 {
		t.Errorf("missing = %v, want default", got)
	}
}

func TestGetStringMapString(t *testing.T) {
	loadEmpty(t)
	SetDefault("headers", map[string]string{"Content-Type": "application/json"})

	got := Get[map[string]string]("headers")
	if got["Content-Type"] != "application/json" {
		t.Errorf("headers = %v", got)
	}

	// map[string]any coercion
	SetDefault("meta", map[string]any{"version": 2, "name": "app"})
	got = Get[map[string]string]("meta")
	if got["version"] != "2" || got["name"] != "app" {
		t.Errorf("meta = %v", got)
	}
}

func TestGetStringMapStringSlice(t *testing.T) {
	loadEmpty(t)
	SetDefault("roles", map[string]any{
		"admin": []any{"read", "write", "delete"},
		"user":  []any{"read"},
	})

	got := Get[map[string][]string]("roles")
	if len(got["admin"]) != 3 || got["admin"][0] != "read" {
		t.Errorf("roles = %v", got)
	}
	if len(got["user"]) != 1 || got["user"][0] != "read" {
		t.Errorf("roles.user = %v", got["user"])
	}
}

// ---------------------------------------------------------------------------
// Ensure
// ---------------------------------------------------------------------------

func TestEnsurePasses(t *testing.T) {
	loadEmpty(t)
	SetDefault("port", 19110)
	Ensure("port") // should not panic
}

func TestEnsurePanics(t *testing.T) {
	loadEmpty(t)
	mustPanic(t, "MISSING_KEY", func() {
		Ensure("missing.key")
	})
}

func TestEnsureAllowsZeroValues(t *testing.T) {
	loadEmpty(t)
	SetDefault("empty_str", "")
	SetDefault("zero_int", 0)
	SetDefault("false_bool", false)

	// None of these should panic — keys exist, zero values are valid
	Ensure("empty_str", "zero_int", "false_bool")
}

func TestEnsureMultipleKeys(t *testing.T) {
	loadEmpty(t)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		msg := fmt.Sprint(r)
		for _, key := range []string{"A", "B", "C"} {
			if !strings.Contains(msg, key) {
				t.Errorf("panic message = %q, missing key %q", msg, key)
			}
		}
	}()
	Ensure("a", "b", "c")
}

func TestEnsurePassesWithEnvVar(t *testing.T) {
	loadEmpty(t)
	t.Setenv("JWT_SECRET", "from-env")
	Ensure("jwt.secret") // should not panic
}

func TestEnsurePassesWithJSON(t *testing.T) {
	loadWithJSON(t, `{"jwt": {"secret": "from-file"}}`)
	Ensure("jwt.secret") // should not panic
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func TestKeyToEnvVar(t *testing.T) {
	cases := map[string]string{
		"port":                    "PORT",
		"data_dir":                "DATA_DIR",
		"jwt.secret":              "JWT_SECRET",
		"db.busy_timeout":         "DB_BUSY_TIMEOUT",
		"cache.max_entries":       "CACHE_MAX_ENTRIES",
		"email.from_address":      "EMAIL_FROM_ADDRESS",
		"multi.deep_inside.level": "MULTI_DEEP_INSIDE_LEVEL",
	}
	for key, want := range cases {
		if got := keyToEnvVar(key); got != want {
			t.Errorf("keyToEnvVar(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestJSONFlattening(t *testing.T) {
	loadWithJSON(t, `{
		"port": 19110,
		"db": {
			"busy_timeout": 5000,
			"synchronous": "NORMAL"
		}
	}`)

	if got := Get[int]("port"); got != 19110 {
		t.Errorf("port = %d, want 19110", got)
	}
	if got := Get[int]("db.busy_timeout"); got != 5000 {
		t.Errorf("db.busy_timeout = %d, want 5000", got)
	}
	if got := Get[string]("db.synchronous"); got != "NORMAL" {
		t.Errorf("db.synchronous = %q, want %q", got, "NORMAL")
	}
}

func TestJSONDeepNesting(t *testing.T) {
	loadWithJSON(t, `{
		"multi": {
			"deep_inside": {
				"level": "value"
			}
		}
	}`)
	if got := Get[string]("multi.deep_inside.level"); got != "value" {
		t.Errorf("got %q, want %q", got, "value")
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestNegativeInt(t *testing.T) {
	loadEmpty(t)
	SetDefault("db.cache_size", -2000)
	if got := Get[int]("db.cache_size"); got != -2000 {
		t.Errorf("got %d, want -2000", got)
	}

	t.Setenv("DB_CACHE_SIZE", "-5000")
	if got := Get[int]("db.cache_size"); got != -5000 {
		t.Errorf("got %d, want -5000", got)
	}
}

func TestZeroValueEnvVar(t *testing.T) {
	loadEmpty(t)
	SetDefault("port", 19110)
	t.Setenv("PORT", "0")
	if got := Get[int]("port"); got != 0 {
		t.Errorf("got %d, want 0 (env override should win)", got)
	}
}

func TestConcurrentAccess(t *testing.T) {
	loadEmpty(t)
	SetDefault("port", 19110)

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			SetDefault(fmt.Sprintf("key.%d", n), n)
		}(i)
		go func(n int) {
			defer wg.Done()
			GetOr[int](fmt.Sprintf("key.%d", n), 0)
		}(i)
	}
	wg.Wait()
}
