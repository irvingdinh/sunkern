package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"sunkern.local/framework/container"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// testModule records lifecycle calls and delegates to injectable functions.
type testModule struct {
	BaseModule
	registerFn func() error
	bootFn     func() error
	shutdownFn func(context.Context) error
	mu         sync.Mutex
	calls      []string
}

func (m *testModule) Register() error {
	m.mu.Lock()
	m.calls = append(m.calls, "register")
	m.mu.Unlock()
	if m.registerFn != nil {
		return m.registerFn()
	}
	return nil
}

func (m *testModule) Boot() error {
	m.mu.Lock()
	m.calls = append(m.calls, "boot")
	m.mu.Unlock()
	if m.bootFn != nil {
		return m.bootFn()
	}
	return nil
}

func (m *testModule) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.calls = append(m.calls, "shutdown")
	m.mu.Unlock()
	if m.shutdownFn != nil {
		return m.shutdownFn(ctx)
	}
	return nil
}

// newTestApp creates an App with an immediate-cancel signal context so Run
// proceeds through boot and immediately begins shutdown without blocking.
func newTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv("HTTP_ADDR", ":0") // random port to avoid conflicts between tests
	container.Reset()
	a := New()
	a.signalCtxFunc = func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // already done — run proceeds to shutdown immediately
		return ctx, cancel
	}
	return a
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
// Lifecycle ordering
// ---------------------------------------------------------------------------

func TestLifecycleOrder(t *testing.T) {
	a := newTestApp(t)

	var mu sync.Mutex
	var events []string
	record := func(s string) {
		mu.Lock()
		events = append(events, s)
		mu.Unlock()
	}

	for _, name := range []string{"m1", "m2", "m3"} {
		n := name
		a.Use(&testModule{
			BaseModule: BaseModule{ModuleName: n},
			registerFn: func() error { record(n + ":register"); return nil },
			bootFn:     func() error { record(n + ":boot"); return nil },
			shutdownFn: func(_ context.Context) error { record(n + ":shutdown"); return nil },
		})
	}

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// All registers before any boot.
	lastRegister := -1
	firstBoot := len(events)
	for i, e := range events {
		if strings.HasSuffix(e, ":register") && i > lastRegister {
			lastRegister = i
		}
		if strings.HasSuffix(e, ":boot") && i < firstBoot {
			firstBoot = i
		}
	}
	if lastRegister >= firstBoot {
		t.Errorf("two-phase violation: events = %v", events)
	}

	// Shutdown in reverse order.
	shutdowns := filterSuffix(events, ":shutdown")
	if len(shutdowns) != 3 || shutdowns[0] != "m3:shutdown" || shutdowns[1] != "m2:shutdown" || shutdowns[2] != "m1:shutdown" {
		t.Errorf("shutdown order = %v, want [m3 m2 m1]", shutdowns)
	}
}

func TestRegisterFailure(t *testing.T) {
	a := newTestApp(t)

	m1 := &testModule{BaseModule: BaseModule{ModuleName: "m1"}}
	m2 := &testModule{
		BaseModule: BaseModule{ModuleName: "m2"},
		registerFn: func() error { return errors.New("m2 register failed") },
	}
	m3 := &testModule{BaseModule: BaseModule{ModuleName: "m3"}}

	a.Use(m1, m2, m3)

	err := a.run()
	if err == nil || !strings.Contains(err.Error(), "m2 register failed") {
		t.Fatalf("expected register error, got: %v", err)
	}

	// m3 should never have been called.
	if contains(m3.calls, "register") {
		t.Error("m3.Register should not have been called")
	}
	// No boot calls at all.
	if contains(m1.calls, "boot") || contains(m2.calls, "boot") {
		t.Error("no module should have booted")
	}
}

func TestBootFailureRollback(t *testing.T) {
	a := newTestApp(t)

	m1 := &testModule{BaseModule: BaseModule{ModuleName: "m1"}}
	m2 := &testModule{
		BaseModule: BaseModule{ModuleName: "m2"},
		bootFn:     func() error { return errors.New("m2 boot failed") },
	}
	m3 := &testModule{BaseModule: BaseModule{ModuleName: "m3"}}

	a.Use(m1, m2, m3)

	err := a.run()
	if err == nil || !strings.Contains(err.Error(), "m2 boot failed") {
		t.Fatalf("expected boot error, got: %v", err)
	}

	// m1 booted successfully → should be shut down.
	if !contains(m1.calls, "shutdown") {
		t.Error("m1 should have been shut down (rollback)")
	}
	// m3 never booted → should NOT be shut down.
	if contains(m3.calls, "boot") {
		t.Error("m3.Boot should not have been called")
	}
	if contains(m3.calls, "shutdown") {
		t.Error("m3.Shutdown should not have been called")
	}
}

func TestShutdownBestEffort(t *testing.T) {
	a := newTestApp(t)

	for _, name := range []string{"m1", "m2", "m3"} {
		n := name
		a.Use(&testModule{
			BaseModule: BaseModule{ModuleName: n},
			shutdownFn: func(_ context.Context) error {
				if n == "m3" || n == "m1" {
					return fmt.Errorf("%s shutdown failed", n)
				}
				return nil
			},
		})
	}

	err := a.run()
	if err == nil {
		t.Fatal("expected shutdown errors")
	}

	msg := err.Error()
	if !strings.Contains(msg, "m3 shutdown failed") || !strings.Contains(msg, "m1 shutdown failed") {
		t.Errorf("error = %q, want both m3 and m1 failures", msg)
	}
}

func TestShutdownReverse(t *testing.T) {
	a := newTestApp(t)

	var mu sync.Mutex
	var order []string
	for i := range 5 {
		n := fmt.Sprintf("m%d", i)
		a.Use(&testModule{
			BaseModule: BaseModule{ModuleName: n},
			shutdownFn: func(_ context.Context) error {
				mu.Lock()
				order = append(order, n)
				mu.Unlock()
				return nil
			},
		})
	}

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	want := []string{"m4", "m3", "m2", "m1", "m0"}
	if len(order) != 5 {
		t.Fatalf("shutdown count = %d, want 5", len(order))
	}
	for i, got := range order {
		if got != want[i] {
			t.Errorf("shutdown[%d] = %q, want %q", i, got, want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Ready state / ShuttingDown
// ---------------------------------------------------------------------------

func TestReadyState(t *testing.T) {
	a := newTestApp(t)

	readyDuringBoot := false
	readyDuringShutdown := true

	a.Use(&testModule{
		BaseModule: BaseModule{ModuleName: "probe"},
		bootFn: func() error {
			readyDuringBoot = a.Ready()
			return nil
		},
		shutdownFn: func(_ context.Context) error {
			readyDuringShutdown = a.Ready()
			return nil
		},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if readyDuringBoot {
		t.Error("Ready should be false during boot")
	}
	if readyDuringShutdown {
		t.Error("Ready should be false during shutdown")
	}
}

func TestShuttingDownChannel(t *testing.T) {
	a := newTestApp(t)

	channelClosed := false
	a.Use(&testModule{
		BaseModule: BaseModule{ModuleName: "probe"},
		shutdownFn: func(_ context.Context) error {
			select {
			case <-a.ShuttingDown():
				channelClosed = true
			default:
			}
			return nil
		},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if !channelClosed {
		t.Error("ShuttingDown channel should be closed during shutdown")
	}
}

// ---------------------------------------------------------------------------
// BaseModule / ModuleGroup
// ---------------------------------------------------------------------------

func TestBaseModuleDefaults(t *testing.T) {
	m := &BaseModule{ModuleName: "test"}
	if m.Name() != "test" {
		t.Errorf("Name = %q, want %q", m.Name(), "test")
	}
	if err := m.Register(); err != nil {
		t.Errorf("Register error = %v, want nil", err)
	}
	if err := m.Boot(); err != nil {
		t.Errorf("Boot error = %v, want nil", err)
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown error = %v, want nil", err)
	}
}

func TestModuleGroup(t *testing.T) {
	var order []string
	record := func(s string) {
		order = append(order, s)
	}

	child := func(name string) *testModule {
		n := name
		return &testModule{
			BaseModule: BaseModule{ModuleName: n},
			registerFn: func() error { record(n + ":register"); return nil },
			bootFn:     func() error { record(n + ":boot"); return nil },
			shutdownFn: func(_ context.Context) error { record(n + ":shutdown"); return nil },
		}
	}

	g := &ModuleGroup{
		GroupName: "admin",
		Modules:   []Module{child("auth"), child("users"), child("logs")},
	}

	if g.Name() != "admin" {
		t.Errorf("Name = %q, want %q", g.Name(), "admin")
	}

	if err := g.Register(); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := g.Boot(); err != nil {
		t.Fatalf("Boot: %v", err)
	}
	if err := g.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// Register and boot forward, shutdown reverse.
	want := []string{
		"auth:register", "users:register", "logs:register",
		"auth:boot", "users:boot", "logs:boot",
		"logs:shutdown", "users:shutdown", "auth:shutdown",
	}
	if len(order) != len(want) {
		t.Fatalf("events = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("events[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestRunCalledTwicePanics(t *testing.T) {
	a := newTestApp(t)
	// First run succeeds (immediately shuts down via cancelled signal ctx).
	if err := a.run(); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	// Second run must panic.
	mustPanic(t, "Run called more than once", func() {
		_ = a.run()
	})
}

func TestNoModules(t *testing.T) {
	a := newTestApp(t)
	if err := a.run(); err != nil {
		t.Fatalf("run with no modules: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Integration with container
// ---------------------------------------------------------------------------

func TestIntegrationWithContainer(t *testing.T) {
	a := newTestApp(t)

	type GreetService struct{ Greeting string }

	var resolved *GreetService

	a.Use(&testModule{
		BaseModule: BaseModule{ModuleName: "provider"},
		registerFn: func() error {
			container.Provide(func() (*GreetService, error) {
				return &GreetService{Greeting: "hello from container"}, nil
			})
			return nil
		},
	})

	a.Use(&testModule{
		BaseModule: BaseModule{ModuleName: "consumer"},
		bootFn: func() error {
			resolved = container.MustMake[*GreetService]()
			return nil
		},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if resolved == nil || resolved.Greeting != "hello from container" {
		t.Errorf("resolved = %v, want greeting %q", resolved, "hello from container")
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func filterSuffix(events []string, suffix string) []string {
	var out []string
	for _, e := range events {
		if strings.HasSuffix(e, suffix) {
			out = append(out, e)
		}
	}
	return out
}

func contains(calls []string, target string) bool {
	for _, c := range calls {
		if c == target {
			return true
		}
	}
	return false
}

// Ensure DATA_DIR is always set for tests that call config.Load via Run.
func TestMain(m *testing.M) {
	if os.Getenv("DATA_DIR") == "" {
		dir, err := os.MkdirTemp("", "sunkern-app-test-*")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(dir)
		os.Setenv("DATA_DIR", dir)
	}
	os.Exit(m.Run())
}
