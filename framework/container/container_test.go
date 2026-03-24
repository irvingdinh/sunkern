package container

import (
	"context"
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

	if err := global.StartHooks(context.Background()); err != nil {
		t.Fatalf("StartHooks failed: %v", err)
	}
	if len(order) != 3 || order[0] != 0 || order[1] != 1 || order[2] != 2 {
		t.Errorf("start order = %v, want [0 1 2]", order)
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

	if err := global.StopHooks(context.Background()); err != nil {
		t.Fatalf("StopHooks failed: %v", err)
	}
	if len(order) != 3 || order[0] != 2 || order[1] != 1 || order[2] != 0 {
		t.Errorf("stop order = %v, want [2 1 0]", order)
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

	err := global.StartHooks(context.Background())
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

	err := global.StopHooks(context.Background())
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

	if err := global.StartHooks(context.Background()); err != nil {
		t.Fatalf("StartHooks with nil callbacks: %v", err)
	}
	if err := global.StopHooks(context.Background()); err != nil {
		t.Fatalf("StopHooks with nil callbacks: %v", err)
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

	err := global.StartHooks(context.Background())
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

	err := global.StopHooks(context.Background())
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
