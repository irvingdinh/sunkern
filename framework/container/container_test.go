package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func resetContainer(t *testing.T) {
	t.Helper()
	Reset()
	t.Cleanup(Reset)
}

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
// Provide / Make
// ---------------------------------------------------------------------------

func TestProvideAndMake(t *testing.T) {
	resetContainer(t)

	type Greeting struct{ Text string }

	Provide(func() (*Greeting, error) {
		return &Greeting{Text: "hello"}, nil
	})

	got, err := Make[*Greeting]()
	if err != nil {
		t.Fatalf("Make failed: %v", err)
	}
	if got.Text != "hello" {
		t.Errorf("got %q, want %q", got.Text, "hello")
	}
}

func TestSupply(t *testing.T) {
	resetContainer(t)

	type Config struct{ Port int }
	cfg := &Config{Port: 19110}
	Supply(cfg)

	got, err := Make[*Config]()
	if err != nil {
		t.Fatalf("Make failed: %v", err)
	}
	if got.Port != 19110 {
		t.Errorf("port = %d, want 19110", got.Port)
	}
	if got != cfg {
		t.Error("expected same pointer from Supply")
	}
}

func TestSingleton(t *testing.T) {
	resetContainer(t)

	var calls atomic.Int32

	type Svc struct{ ID int }
	Provide(func() (*Svc, error) {
		n := calls.Add(1)
		return &Svc{ID: int(n)}, nil
	})

	a := MustMake[*Svc]()
	b := MustMake[*Svc]()

	if a != b {
		t.Error("expected same instance (singleton)")
	}
	if calls.Load() != 1 {
		t.Errorf("provider called %d times, want 1", calls.Load())
	}
}

func TestTransitiveDeps(t *testing.T) {
	resetContainer(t)

	type Config struct{ DSN string }
	type DB struct{ DSN string }
	type Repo struct{ DB *DB }

	Supply(&Config{DSN: "file:test.db"})

	Provide(func() (*DB, error) {
		cfg := MustMake[*Config]()
		return &DB{DSN: cfg.DSN}, nil
	})

	Provide(func() (*Repo, error) {
		db := MustMake[*DB]()
		return &Repo{DB: db}, nil
	})

	repo := MustMake[*Repo]()
	if repo.DB.DSN != "file:test.db" {
		t.Errorf("repo.DB.DSN = %q, want %q", repo.DB.DSN, "file:test.db")
	}
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestDuplicatePanics(t *testing.T) {
	resetContainer(t)

	type Svc struct{}
	Provide(func() (*Svc, error) { return &Svc{}, nil })

	mustPanic(t, "duplicate provider", func() {
		Provide(func() (*Svc, error) { return &Svc{}, nil })
	})
}

func TestDuplicateSupplyPanics(t *testing.T) {
	resetContainer(t)

	type Cfg struct{}
	Supply(&Cfg{})

	mustPanic(t, "duplicate provider", func() {
		Supply(&Cfg{})
	})
}

func TestDuplicatePanicIncludesCaller(t *testing.T) {
	resetContainer(t)

	type Svc struct{}
	Supply(&Svc{})

	mustPanic(t, "container_test.go:", func() {
		Supply(&Svc{})
	})
}

func TestMakeError(t *testing.T) {
	resetContainer(t)

	type Broken struct{}
	Provide(func() (*Broken, error) {
		return nil, errors.New("connection refused")
	})

	_, err := Make[*Broken]()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error = %q, want substring %q", err.Error(), "connection refused")
	}
}

func TestMustMakePanicsOnMissing(t *testing.T) {
	resetContainer(t)

	type Missing struct{}
	mustPanic(t, "service not found", func() {
		MustMake[*Missing]()
	})
}

func TestMustMakePanicsOnError(t *testing.T) {
	resetContainer(t)

	type Broken struct{}
	Provide(func() (*Broken, error) {
		return nil, errors.New("boom")
	})

	mustPanic(t, "boom", func() {
		MustMake[*Broken]()
	})
}

// ---------------------------------------------------------------------------
// Has / Reset
// ---------------------------------------------------------------------------

func TestHas(t *testing.T) {
	resetContainer(t)

	type Svc struct{}
	if Has[*Svc]() {
		t.Error("Has should return false before registration")
	}

	Provide(func() (*Svc, error) { return &Svc{}, nil })

	if !Has[*Svc]() {
		t.Error("Has should return true after registration")
	}
}

func TestReset(t *testing.T) {
	resetContainer(t)

	type Svc struct{}
	Provide(func() (*Svc, error) { return &Svc{}, nil })

	Reset()

	if Has[*Svc]() {
		t.Error("Has should return false after Reset")
	}
}

func TestDifferentTypes(t *testing.T) {
	resetContainer(t)

	Supply(42)
	Supply("hello")

	gotInt := MustMake[int]()
	gotStr := MustMake[string]()

	if gotInt != 42 {
		t.Errorf("int = %d, want 42", gotInt)
	}
	if gotStr != "hello" {
		t.Errorf("string = %q, want %q", gotStr, "hello")
	}
}

// ---------------------------------------------------------------------------
// Hooks
// ---------------------------------------------------------------------------

func TestStartHooksOrder(t *testing.T) {
	resetContainer(t)

	var order []int
	for i := range 3 {
		n := i
		AppendHook(Hook{
			OnStart: func(_ context.Context) error {
				order = append(order, n)
				return nil
			},
		})
	}

	reports, err := global.StartHooks(context.Background())
	if err != nil {
		t.Fatalf("StartHooks failed: %v", err)
	}
	if len(order) != 3 || order[0] != 0 || order[1] != 1 || order[2] != 2 {
		t.Errorf("start order = %v, want [0 1 2]", order)
	}
	if len(reports) != 3 {
		t.Errorf("reports count = %d, want 3", len(reports))
	}
}

func TestStopHooksReverse(t *testing.T) {
	resetContainer(t)

	var order []int
	for i := range 3 {
		n := i
		AppendHook(Hook{
			OnStop: func(_ context.Context) error {
				order = append(order, n)
				return nil
			},
		})
	}

	reports, err := global.StopHooks(context.Background())
	if err != nil {
		t.Fatalf("StopHooks failed: %v", err)
	}
	if len(order) != 3 || order[0] != 2 || order[1] != 1 || order[2] != 0 {
		t.Errorf("stop order = %v, want [2 1 0]", order)
	}
	if len(reports) != 3 {
		t.Errorf("reports count = %d, want 3", len(reports))
	}
}

func TestStartHookRollback(t *testing.T) {
	resetContainer(t)

	var started, stopped []int

	for i := range 3 {
		n := i
		AppendHook(Hook{
			OnStart: func(_ context.Context) error {
				if n == 2 {
					return fmt.Errorf("hook %d failed", n)
				}
				started = append(started, n)
				return nil
			},
			OnStop: func(_ context.Context) error {
				stopped = append(stopped, n)
				return nil
			},
		})
	}

	reports, err := global.StartHooks(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	// Hooks 0 and 1 started successfully; they should be stopped in reverse.
	if len(started) != 2 || started[0] != 0 || started[1] != 1 {
		t.Errorf("started = %v, want [0 1]", started)
	}
	if len(stopped) != 2 || stopped[0] != 1 || stopped[1] != 0 {
		t.Errorf("stopped = %v, want [1 0]", stopped)
	}

	// Reports cover 3 hooks: 0 (ok), 1 (ok), 2 (failed).
	if len(reports) != 3 {
		t.Errorf("reports count = %d, want 3", len(reports))
	}
	if len(reports) == 3 && reports[2].Err == "" {
		t.Error("expected error in last report")
	}
}

func TestStopHooksBestEffort(t *testing.T) {
	resetContainer(t)

	var stopped []int

	for i := range 3 {
		n := i
		AppendHook(Hook{
			OnStop: func(_ context.Context) error {
				stopped = append(stopped, n)
				if n == 2 || n == 0 {
					return fmt.Errorf("stop %d failed", n)
				}
				return nil
			},
		})
	}

	_, err := global.StopHooks(context.Background())
	if err == nil {
		t.Fatal("expected error from stop hooks")
	}

	// All three hooks must have been attempted despite errors.
	if len(stopped) != 3 {
		t.Errorf("stopped = %v, want all 3 attempted", stopped)
	}

	msg := err.Error()
	if !strings.Contains(msg, "stop 2 failed") || !strings.Contains(msg, "stop 0 failed") {
		t.Errorf("error = %q, want both failures", msg)
	}
}

func TestHooksNilCallbacksSkipped(t *testing.T) {
	resetContainer(t)

	AppendHook(Hook{OnStart: nil, OnStop: nil})

	reports, err := global.StartHooks(context.Background())
	if err != nil {
		t.Fatalf("StartHooks with nil callbacks: %v", err)
	}
	if len(reports) != 0 {
		t.Errorf("expected 0 reports for nil callbacks, got %d", len(reports))
	}

	reports, err = global.StopHooks(context.Background())
	if err != nil {
		t.Fatalf("StopHooks with nil callbacks: %v", err)
	}
	if len(reports) != 0 {
		t.Errorf("expected 0 reports for nil callbacks, got %d", len(reports))
	}
}

func TestMakeErrorCached(t *testing.T) {
	resetContainer(t)

	var calls atomic.Int32
	type Flaky struct{}
	Provide(func() (*Flaky, error) {
		calls.Add(1)
		return nil, errors.New("connection refused")
	})

	// First call: provider runs, fails.
	_, err1 := Make[*Flaky]()
	if err1 == nil {
		t.Fatal("expected error on first call")
	}
	// Second call: cached error returned, provider NOT called again.
	_, err2 := Make[*Flaky]()
	if err2 == nil {
		t.Fatal("expected error on second call")
	}
	if calls.Load() != 1 {
		t.Errorf("provider called %d times, want 1 (error should be cached)", calls.Load())
	}
}

// ---------------------------------------------------------------------------
// Override
// ---------------------------------------------------------------------------

func TestOverride(t *testing.T) {
	resetContainer(t)

	type Svc struct{ Name string }
	Provide(func() (*Svc, error) { return &Svc{Name: "original"}, nil })

	Override(func() (*Svc, error) { return &Svc{Name: "replaced"}, nil })

	got := MustMake[*Svc]()
	if got.Name != "replaced" {
		t.Errorf("Name = %q, want %q", got.Name, "replaced")
	}
}

func TestOverrideAfterBuild(t *testing.T) {
	resetContainer(t)

	type Svc struct{ Name string }
	Provide(func() (*Svc, error) { return &Svc{Name: "original"}, nil })

	// Build the original.
	orig := MustMake[*Svc]()
	if orig.Name != "original" {
		t.Fatalf("Name = %q, want %q", orig.Name, "original")
	}

	// Override discards the cached instance.
	Override(func() (*Svc, error) { return &Svc{Name: "replaced"}, nil })

	got := MustMake[*Svc]()
	if got.Name != "replaced" {
		t.Errorf("Name = %q, want %q", got.Name, "replaced")
	}
	if got == orig {
		t.Error("expected different instance after Override")
	}
}

func TestOverrideSupply(t *testing.T) {
	resetContainer(t)

	type Cfg struct{ Port int }
	Supply(&Cfg{Port: 3000})

	OverrideSupply(&Cfg{Port: 8080})

	got := MustMake[*Cfg]()
	if got.Port != 8080 {
		t.Errorf("Port = %d, want 8080", got.Port)
	}
}

func TestOverrideUnregistered(t *testing.T) {
	resetContainer(t)

	type Svc struct{ Name string }
	// Override on an unregistered type should work like Provide.
	Override(func() (*Svc, error) { return &Svc{Name: "new"}, nil })

	got := MustMake[*Svc]()
	if got.Name != "new" {
		t.Errorf("Name = %q, want %q", got.Name, "new")
	}
}

// ---------------------------------------------------------------------------
// Named hooks
// ---------------------------------------------------------------------------

func TestNamedHookErrorMessage(t *testing.T) {
	resetContainer(t)

	AppendHook(Hook{
		Name: "database",
		OnStart: func(_ context.Context) error {
			return fmt.Errorf("connection refused")
		},
	})

	_, err := global.StartHooks(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `"database"`) {
		t.Errorf("error = %q, want hook name %q in message", err.Error(), "database")
	}
}

func TestNamedHookStopErrorMessage(t *testing.T) {
	resetContainer(t)

	AppendHook(Hook{
		Name: "cache",
		OnStop: func(_ context.Context) error {
			return fmt.Errorf("flush failed")
		},
	})

	_, err := global.StopHooks(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `"cache"`) {
		t.Errorf("error = %q, want hook name %q in message", err.Error(), "cache")
	}
}

// ---------------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------------

func TestConcurrentMake(t *testing.T) {
	resetContainer(t)

	var calls atomic.Int32

	type Svc struct{ ID int32 }
	Provide(func() (*Svc, error) {
		n := calls.Add(1)
		return &Svc{ID: n}, nil
	})

	var wg sync.WaitGroup
	results := make([]*Svc, 100)
	for i := range 100 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = MustMake[*Svc]()
		}(i)
	}
	wg.Wait()

	if calls.Load() != 1 {
		t.Errorf("provider called %d times, want 1", calls.Load())
	}

	first := results[0]
	for i, r := range results {
		if r != first {
			t.Errorf("results[%d] is a different instance", i)
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Introspection: Len, Keys, Inspect
// ---------------------------------------------------------------------------

func TestLen(t *testing.T) {
	resetContainer(t)

	if Len() != 0 {
		t.Errorf("Len = %d, want 0", Len())
	}

	Supply(42)
	Supply("hello")

	if Len() != 2 {
		t.Errorf("Len = %d, want 2", Len())
	}
}

func TestKeys(t *testing.T) {
	resetContainer(t)

	type Alpha struct{}
	type Beta struct{}

	Supply(&Beta{})
	Supply(&Alpha{})
	Supply(42)

	keys := Keys()
	if len(keys) != 3 {
		t.Fatalf("Keys count = %d, want 3", len(keys))
	}
	// Must be sorted.
	for i := 1; i < len(keys); i++ {
		if keys[i] < keys[i-1] {
			t.Errorf("Keys not sorted: %v", keys)
			break
		}
	}
}

func TestKeysEmpty(t *testing.T) {
	resetContainer(t)

	keys := Keys()
	if len(keys) != 0 {
		t.Errorf("Keys = %v, want empty", keys)
	}
}

func TestInspectPending(t *testing.T) {
	resetContainer(t)

	type Svc struct{}
	Provide(func() (*Svc, error) { return &Svc{}, nil })

	infos := Inspect()
	if len(infos) != 1 {
		t.Fatalf("Inspect count = %d, want 1", len(infos))
	}
	if infos[0].Status != ServicePending {
		t.Errorf("Status = %v, want Pending", infos[0].Status)
	}
	if infos[0].Kind != "provided" {
		t.Errorf("Kind = %q, want %q", infos[0].Kind, "provided")
	}
	if infos[0].Error != nil {
		t.Errorf("Error = %v, want nil", infos[0].Error)
	}
}

func TestInspectBuilt(t *testing.T) {
	resetContainer(t)

	type Svc struct{}
	Provide(func() (*Svc, error) { return &Svc{}, nil })
	_ = MustMake[*Svc]() // trigger build

	infos := Inspect()
	if len(infos) != 1 {
		t.Fatalf("Inspect count = %d, want 1", len(infos))
	}
	if infos[0].Status != ServiceBuilt {
		t.Errorf("Status = %v, want Built", infos[0].Status)
	}
}

func TestInspectFailed(t *testing.T) {
	resetContainer(t)

	type Broken struct{}
	Provide(func() (*Broken, error) { return nil, errors.New("broken") })
	_, _ = Make[*Broken]() // trigger build (and fail)

	infos := Inspect()
	if len(infos) != 1 {
		t.Fatalf("Inspect count = %d, want 1", len(infos))
	}
	if infos[0].Status != ServiceFailed {
		t.Errorf("Status = %v, want Failed", infos[0].Status)
	}
	if infos[0].Error == nil || !strings.Contains(infos[0].Error.Error(), "broken") {
		t.Errorf("Error = %v, want 'broken'", infos[0].Error)
	}
	if infos[0].ErrorText != "broken" {
		t.Errorf("ErrorText = %q, want %q", infos[0].ErrorText, "broken")
	}
}

func TestInspectSorted(t *testing.T) {
	resetContainer(t)

	type Zebra struct{}
	type Apple struct{}
	Supply(&Zebra{})
	Supply(&Apple{})

	infos := Inspect()
	if len(infos) != 2 {
		t.Fatalf("Inspect count = %d, want 2", len(infos))
	}
	if infos[0].Name >= infos[1].Name {
		t.Errorf("Inspect not sorted: [%s, %s]", infos[0].Name, infos[1].Name)
	}
}

func TestInspectDoesNotTriggerBuild(t *testing.T) {
	resetContainer(t)

	var calls atomic.Int32
	type Svc struct{}
	Provide(func() (*Svc, error) {
		calls.Add(1)
		return &Svc{}, nil
	})

	_ = Inspect() // should not call provider

	if calls.Load() != 0 {
		t.Errorf("Inspect triggered provider (calls = %d)", calls.Load())
	}
}

func TestServiceStatusString(t *testing.T) {
	cases := []struct {
		s    ServiceStatus
		want string
	}{
		{ServicePending, "pending"},
		{ServiceBuilt, "built"},
		{ServiceFailed, "failed"},
		{ServiceStatus(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("ServiceStatus(%d).String() = %q, want %q", c.s, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Hooks: Hooks() accessor
// ---------------------------------------------------------------------------

func TestHooksAccessor(t *testing.T) {
	resetContainer(t)

	AppendHook(Hook{Name: "a"})
	AppendHook(Hook{Name: "b"})

	hooks := global.Hooks()
	if len(hooks) != 2 {
		t.Fatalf("Hooks count = %d, want 2", len(hooks))
	}
	if hooks[0].Name != "a" || hooks[1].Name != "b" {
		t.Errorf("Hooks = [%s, %s], want [a, b]", hooks[0].Name, hooks[1].Name)
	}

	// Returned slice must be a copy — mutating it shouldn't affect container.
	hooks[0].Name = "mutated"
	hooks2 := global.Hooks()
	if hooks2[0].Name != "a" {
		t.Error("Hooks returned a reference, not a copy")
	}
}

func TestInspectSupplied(t *testing.T) {
	resetContainer(t)

	Supply(42)

	infos := Inspect()
	if len(infos) != 1 {
		t.Fatalf("Inspect count = %d, want 1", len(infos))
	}
	if infos[0].Status != ServiceBuilt {
		t.Errorf("Supplied service status = %v, want Built", infos[0].Status)
	}
	if infos[0].Kind != "supplied" {
		t.Errorf("Kind = %q, want %q", infos[0].Kind, "supplied")
	}
}

// ---------------------------------------------------------------------------
// ServiceInfo: Kind, Caller, JSON
// ---------------------------------------------------------------------------

func TestInspectCaller(t *testing.T) {
	resetContainer(t)

	type Svc struct{}
	Provide(func() (*Svc, error) { return &Svc{}, nil })

	infos := Inspect()
	if len(infos) != 1 {
		t.Fatalf("Inspect count = %d, want 1", len(infos))
	}
	// Caller should contain "container_test.go:" since we registered from this file.
	if !strings.Contains(infos[0].Caller, "container_test.go:") {
		t.Errorf("Caller = %q, want to contain %q", infos[0].Caller, "container_test.go:")
	}
}

func TestInspectKindProvided(t *testing.T) {
	resetContainer(t)

	type Svc struct{}
	Provide(func() (*Svc, error) { return &Svc{}, nil })

	infos := Inspect()
	if len(infos) != 1 {
		t.Fatalf("Inspect count = %d, want 1", len(infos))
	}
	if infos[0].Kind != "provided" {
		t.Errorf("Kind = %q, want %q", infos[0].Kind, "provided")
	}
}

func TestInspectKindSupplied(t *testing.T) {
	resetContainer(t)

	Supply(42)

	infos := Inspect()
	if len(infos) != 1 {
		t.Fatalf("Inspect count = %d, want 1", len(infos))
	}
	if infos[0].Kind != "supplied" {
		t.Errorf("Kind = %q, want %q", infos[0].Kind, "supplied")
	}
}

func TestOverrideUpdatesCaller(t *testing.T) {
	resetContainer(t)

	type Svc struct{}
	Provide(func() (*Svc, error) { return &Svc{}, nil })

	infos1 := Inspect()
	caller1 := infos1[0].Caller

	Override(func() (*Svc, error) { return &Svc{}, nil })

	infos2 := Inspect()
	caller2 := infos2[0].Caller

	// Both should reference this test file, but potentially different lines.
	if !strings.Contains(caller1, "container_test.go:") {
		t.Errorf("caller1 = %q, want container_test.go", caller1)
	}
	if !strings.Contains(caller2, "container_test.go:") {
		t.Errorf("caller2 = %q, want container_test.go", caller2)
	}
}

func TestOverrideUpdatesKind(t *testing.T) {
	resetContainer(t)

	type Svc struct{ V int }
	Provide(func() (*Svc, error) { return &Svc{V: 1}, nil })

	infos := Inspect()
	if infos[0].Kind != "provided" {
		t.Errorf("Kind = %q, want %q", infos[0].Kind, "provided")
	}

	OverrideSupply(&Svc{V: 2})

	infos = Inspect()
	if infos[0].Kind != "supplied" {
		t.Errorf("Kind after OverrideSupply = %q, want %q", infos[0].Kind, "supplied")
	}
}

func TestServiceStatusMarshalJSON(t *testing.T) {
	cases := []struct {
		s    ServiceStatus
		want string
	}{
		{ServicePending, `"pending"`},
		{ServiceBuilt, `"built"`},
		{ServiceFailed, `"failed"`},
	}
	for _, c := range cases {
		got, err := json.Marshal(c.s)
		if err != nil {
			t.Fatalf("Marshal(%v) error: %v", c.s, err)
		}
		if string(got) != c.want {
			t.Errorf("Marshal(%v) = %s, want %s", c.s, got, c.want)
		}
	}
}

func TestServiceInfoJSON(t *testing.T) {
	resetContainer(t)

	type Broken struct{}
	Provide(func() (*Broken, error) { return nil, errors.New("oops") })
	_, _ = Make[*Broken]()

	infos := Inspect()
	if len(infos) != 1 {
		t.Fatalf("Inspect count = %d, want 1", len(infos))
	}

	data, err := json.Marshal(infos[0])
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if m["status"] != "failed" {
		t.Errorf("JSON status = %v, want %q", m["status"], "failed")
	}
	if m["kind"] != "provided" {
		t.Errorf("JSON kind = %v, want %q", m["kind"], "provided")
	}
	if m["error"] != "oops" {
		t.Errorf("JSON error = %v, want %q", m["error"], "oops")
	}
	if _, ok := m["caller"]; !ok {
		t.Error("JSON missing caller field")
	}
}

// ---------------------------------------------------------------------------
// HookReport
// ---------------------------------------------------------------------------

func TestHookReportTiming(t *testing.T) {
	resetContainer(t)

	AppendHook(Hook{
		Name: "fast",
		OnStart: func(_ context.Context) error {
			return nil
		},
	})

	reports, err := global.StartHooks(context.Background())
	if err != nil {
		t.Fatalf("StartHooks error: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("reports count = %d, want 1", len(reports))
	}
	if reports[0].Name != "fast" {
		t.Errorf("report name = %q, want %q", reports[0].Name, "fast")
	}
	if reports[0].DurationMs < 0 {
		t.Errorf("DurationMs = %d, want >= 0", reports[0].DurationMs)
	}
	if reports[0].Err != "" {
		t.Errorf("report err = %q, want empty", reports[0].Err)
	}
}

func TestHookReportJSON(t *testing.T) {
	report := HookReport{
		Name:       "sqlite",
		DurationMs: 42,
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if m["name"] != "sqlite" {
		t.Errorf("JSON name = %v, want %q", m["name"], "sqlite")
	}
	if m["duration_ms"] != float64(42) {
		t.Errorf("JSON duration_ms = %v, want 42", m["duration_ms"])
	}
	// error field should be omitted (empty)
	if _, ok := m["error"]; ok {
		t.Error("JSON should omit empty error field")
	}
}

func TestStopHookReportsIncludeErrors(t *testing.T) {
	resetContainer(t)

	AppendHook(Hook{
		Name: "ok-hook",
		OnStop: func(_ context.Context) error {
			return nil
		},
	})
	AppendHook(Hook{
		Name: "fail-hook",
		OnStop: func(_ context.Context) error {
			return fmt.Errorf("cleanup failed")
		},
	})

	reports, err := global.StopHooks(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if len(reports) != 2 {
		t.Fatalf("reports count = %d, want 2", len(reports))
	}

	// Reports are in reverse order (fail-hook first, then ok-hook).
	if reports[0].Name != "fail-hook" {
		t.Errorf("first report name = %q, want %q", reports[0].Name, "fail-hook")
	}
	if reports[0].Err == "" {
		t.Error("expected error in fail-hook report")
	}
	if reports[1].Name != "ok-hook" {
		t.Errorf("second report name = %q, want %q", reports[1].Name, "ok-hook")
	}
	if reports[1].Err != "" {
		t.Errorf("ok-hook should have no error, got %q", reports[1].Err)
	}
}

// ---------------------------------------------------------------------------
// Circular dependency detection
// ---------------------------------------------------------------------------

func TestCircularDepSelf(t *testing.T) {
	resetContainer(t)

	type SelfRef struct{}
	Provide(func() (*SelfRef, error) {
		_ = MustMake[*SelfRef]() // self-dependency
		return &SelfRef{}, nil
	})

	mustPanic(t, "circular dependency", func() {
		MustMake[*SelfRef]()
	})
}

func TestCircularDepTwoServices(t *testing.T) {
	resetContainer(t)

	type A struct{}
	type B struct{}

	Provide(func() (*A, error) {
		_ = MustMake[*B]()
		return &A{}, nil
	})
	Provide(func() (*B, error) {
		_ = MustMake[*A]()
		return &B{}, nil
	})

	mustPanic(t, "circular dependency", func() {
		MustMake[*A]()
	})
}

func TestCircularDepThreeServices(t *testing.T) {
	resetContainer(t)

	type X struct{}
	type Y struct{}
	type Z struct{}

	Provide(func() (*X, error) {
		_ = MustMake[*Y]()
		return &X{}, nil
	})
	Provide(func() (*Y, error) {
		_ = MustMake[*Z]()
		return &Y{}, nil
	})
	Provide(func() (*Z, error) {
		_ = MustMake[*X]()
		return &Z{}, nil
	})

	mustPanic(t, "circular dependency", func() {
		MustMake[*X]()
	})
}

func TestCircularDepChainInMessage(t *testing.T) {
	resetContainer(t)

	type Alpha struct{}
	type Beta struct{}

	Provide(func() (*Alpha, error) {
		_ = MustMake[*Beta]()
		return &Alpha{}, nil
	})
	Provide(func() (*Beta, error) {
		_ = MustMake[*Alpha]()
		return &Beta{}, nil
	})

	// Verify the panic message contains the cycle chain with arrows.
	mustPanic(t, "→", func() {
		MustMake[*Alpha]()
	})
}

// ---------------------------------------------------------------------------
// Dependency tracking & graph
// ---------------------------------------------------------------------------

func TestDependencyTracking(t *testing.T) {
	resetContainer(t)

	type Config struct{ DSN string }
	type DB struct{ DSN string }
	type Repo struct{ DB *DB }

	Supply(&Config{DSN: "file:test.db"})

	Provide(func() (*DB, error) {
		cfg := MustMake[*Config]()
		return &DB{DSN: cfg.DSN}, nil
	})

	Provide(func() (*Repo, error) {
		db := MustMake[*DB]()
		return &Repo{DB: db}, nil
	})

	_ = MustMake[*Repo]()

	infos := Inspect()
	// Find each service's deps.
	depsByName := make(map[string][]string)
	for _, info := range infos {
		depsByName[info.Name] = info.Deps
	}

	// Config: supplied, no deps.
	if deps := depsByName["*container.Config"]; len(deps) != 0 {
		t.Errorf("Config deps = %v, want empty", deps)
	}

	// DB: depends on Config.
	dbDeps := depsByName["*container.DB"]
	if len(dbDeps) != 1 || dbDeps[0] != "*container.Config" {
		t.Errorf("DB deps = %v, want [*container.Config]", dbDeps)
	}

	// Repo: depends on DB (not transitively on Config).
	repoDeps := depsByName["*container.Repo"]
	if len(repoDeps) != 1 || repoDeps[0] != "*container.DB" {
		t.Errorf("Repo deps = %v, want [*container.DB]", repoDeps)
	}
}

func TestDependencyGraph(t *testing.T) {
	resetContainer(t)

	type A struct{}
	type B struct{}
	type C struct{}

	Supply(&A{})
	Provide(func() (*B, error) {
		_ = MustMake[*A]()
		return &B{}, nil
	})
	Provide(func() (*C, error) {
		_ = MustMake[*A]()
		_ = MustMake[*B]()
		return &C{}, nil
	})

	_ = MustMake[*C]()

	graph := DependencyGraph()

	// A: supplied, no deps → not in graph.
	if _, ok := graph["*container.A"]; ok {
		t.Error("A should not be in DependencyGraph (no deps)")
	}

	// B: depends on A.
	bDeps := graph["*container.B"]
	if len(bDeps) != 1 || bDeps[0] != "*container.A" {
		t.Errorf("B graph = %v, want [*container.A]", bDeps)
	}

	// C: depends on A and B.
	cDeps := graph["*container.C"]
	if len(cDeps) != 2 || cDeps[0] != "*container.A" || cDeps[1] != "*container.B" {
		t.Errorf("C graph = %v, want [*container.A, *container.B]", cDeps)
	}
}

func TestDependencyGraphEmpty(t *testing.T) {
	resetContainer(t)

	graph := DependencyGraph()
	if len(graph) != 0 {
		t.Errorf("DependencyGraph = %v, want empty", graph)
	}
}

func TestDependencyDeduplication(t *testing.T) {
	resetContainer(t)

	type Dep struct{}
	type Svc struct{}

	Supply(&Dep{})
	Provide(func() (*Svc, error) {
		// Resolve same dep twice.
		_ = MustMake[*Dep]()
		_ = MustMake[*Dep]()
		return &Svc{}, nil
	})

	_ = MustMake[*Svc]()

	infos := Inspect()
	for _, info := range infos {
		if info.Name == "*container.Svc" {
			if len(info.Deps) != 1 {
				t.Errorf("Svc deps = %v, want exactly 1 (deduplicated)", info.Deps)
			}
			return
		}
	}
	t.Fatal("Svc not found in Inspect()")
}

// ---------------------------------------------------------------------------
// Container.Reset (instance method)
// ---------------------------------------------------------------------------

func TestContainerReset(t *testing.T) {
	c := New()

	type Svc struct{}
	provideToContainer[*Svc](c, func() (*Svc, error) { return &Svc{}, nil }, "test")
	c.AppendHook(Hook{Name: "test-hook"})

	if c.Len() != 1 {
		t.Fatalf("Len = %d, want 1", c.Len())
	}
	if len(c.Hooks()) != 1 {
		t.Fatalf("Hooks count = %d, want 1", len(c.Hooks()))
	}

	c.Reset()

	if c.Len() != 0 {
		t.Errorf("Len after Reset = %d, want 0", c.Len())
	}
	if len(c.Hooks()) != 0 {
		t.Errorf("Hooks after Reset count = %d, want 0", len(c.Hooks()))
	}
}

func TestDependencyTrackingJSON(t *testing.T) {
	resetContainer(t)

	type Dep struct{}
	type Svc struct{}

	Supply(&Dep{})
	Provide(func() (*Svc, error) {
		_ = MustMake[*Dep]()
		return &Svc{}, nil
	})
	_ = MustMake[*Svc]()

	infos := Inspect()
	for _, info := range infos {
		if info.Name == "*container.Svc" {
			data, err := json.Marshal(info)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}
			deps, ok := m["deps"].([]any)
			if !ok || len(deps) != 1 {
				t.Errorf("JSON deps = %v, want 1-element array", m["deps"])
			}
			return
		}
	}
	t.Fatal("Svc not found in Inspect()")
}
