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
	sunkernlog "sunkern.local/framework/log"
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

// testModuleWithPostBoot extends testModule with the PostBooter interface.
type testModuleWithPostBoot struct {
	testModule
	postBootFn func() error
}

func (m *testModuleWithPostBoot) PostBoot() error {
	m.mu.Lock()
	m.calls = append(m.calls, "post-boot")
	m.mu.Unlock()
	if m.postBootFn != nil {
		return m.postBootFn()
	}
	return nil
}

// testModuleWithPreShutdown extends testModule with the PreShutdowner interface.
type testModuleWithPreShutdown struct {
	testModule
	preShutdownFn func(context.Context) error
}

func (m *testModuleWithPreShutdown) PreShutdown(ctx context.Context) error {
	m.mu.Lock()
	m.calls = append(m.calls, "pre-shutdown")
	m.mu.Unlock()
	if m.preShutdownFn != nil {
		return m.preShutdownFn(ctx)
	}
	return nil
}

// testModuleWithBothHooks extends testModule with PostBooter and PreShutdowner.
type testModuleWithBothHooks struct {
	testModule
	postBootFn    func() error
	preShutdownFn func(context.Context) error
}

func (m *testModuleWithBothHooks) PostBoot() error {
	m.mu.Lock()
	m.calls = append(m.calls, "post-boot")
	m.mu.Unlock()
	if m.postBootFn != nil {
		return m.postBootFn()
	}
	return nil
}

func (m *testModuleWithBothHooks) PreShutdown(ctx context.Context) error {
	m.mu.Lock()
	m.calls = append(m.calls, "pre-shutdown")
	m.mu.Unlock()
	if m.preShutdownFn != nil {
		return m.preShutdownFn(ctx)
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
	sunkernlog.Reset()
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
// ModuleGroup boot rollback
// ---------------------------------------------------------------------------

func TestModuleGroupBootRollback(t *testing.T) {
	// When a child module fails Boot inside a ModuleGroup, previously-booted
	// children must be shut down (rollback).
	var order []string
	record := func(s string) { order = append(order, s) }

	g := &ModuleGroup{
		GroupName: "admin",
		Modules: []Module{
			&testModule{
				BaseModule: BaseModule{ModuleName: "auth"},
				bootFn:     func() error { record("auth:boot"); return nil },
				shutdownFn: func(_ context.Context) error { record("auth:shutdown"); return nil },
			},
			&testModule{
				BaseModule: BaseModule{ModuleName: "users"},
				bootFn:     func() error { record("users:boot"); return nil },
				shutdownFn: func(_ context.Context) error { record("users:shutdown"); return nil },
			},
			&testModule{
				BaseModule: BaseModule{ModuleName: "logs"},
				bootFn:     func() error { return fmt.Errorf("logs boot failed") },
				shutdownFn: func(_ context.Context) error { record("logs:shutdown"); return nil },
			},
			&testModule{
				BaseModule: BaseModule{ModuleName: "settings"},
				bootFn:     func() error { record("settings:boot"); return nil },
				shutdownFn: func(_ context.Context) error { record("settings:shutdown"); return nil },
			},
		},
	}

	err := g.Boot()
	if err == nil || !strings.Contains(err.Error(), "logs boot failed") {
		t.Fatalf("expected boot error, got: %v", err)
	}

	// auth and users booted, then logs failed.
	// settings should never have booted.
	// auth and users should be shut down in reverse.
	want := []string{
		"auth:boot", "users:boot",
		"users:shutdown", "auth:shutdown",
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

func TestModuleGroupShutdownOnlyBooted(t *testing.T) {
	// Shutdown should only shut down children that completed Boot.
	var shutdowns []string

	g := &ModuleGroup{
		GroupName: "group",
		Modules: []Module{
			&testModule{
				BaseModule: BaseModule{ModuleName: "a"},
				shutdownFn: func(_ context.Context) error { shutdowns = append(shutdowns, "a"); return nil },
			},
			&testModule{
				BaseModule: BaseModule{ModuleName: "b"},
				bootFn:     func() error { return fmt.Errorf("b failed") },
				shutdownFn: func(_ context.Context) error { shutdowns = append(shutdowns, "b"); return nil },
			},
		},
	}

	// Boot fails at b → only a booted.
	_ = g.Boot()

	// Reset and call Shutdown explicitly — only a should be shut down.
	// But a was already rolled back during Boot failure. After rollback,
	// booted is empty, so Shutdown should be a no-op.
	shutdowns = nil
	_ = g.Shutdown(context.Background())
	if len(shutdowns) != 0 {
		t.Errorf("Shutdown after failed Boot should be no-op, got %v", shutdowns)
	}
}

// ---------------------------------------------------------------------------
// PostBoot / PreShutdown
// ---------------------------------------------------------------------------

func TestPostBoot(t *testing.T) {
	a := newTestApp(t)

	m := &testModuleWithPostBoot{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "m1"}},
	}
	a.Use(m)

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if !contains(m.calls, "post-boot") {
		t.Error("PostBoot should have been called")
	}
}

func TestPostBootAfterAllBoot(t *testing.T) {
	a := newTestApp(t)

	var mu sync.Mutex
	var events []string
	record := func(s string) {
		mu.Lock()
		events = append(events, s)
		mu.Unlock()
	}

	// m1: regular module (no PostBoot)
	a.Use(&testModule{
		BaseModule: BaseModule{ModuleName: "m1"},
		bootFn:     func() error { record("m1:boot"); return nil },
	})

	// m2: has PostBoot
	a.Use(&testModuleWithPostBoot{
		testModule: testModule{
			BaseModule: BaseModule{ModuleName: "m2"},
			bootFn:     func() error { record("m2:boot"); return nil },
		},
		postBootFn: func() error { record("m2:post-boot"); return nil },
	})

	// m3: regular module (no PostBoot)
	a.Use(&testModule{
		BaseModule: BaseModule{ModuleName: "m3"},
		bootFn:     func() error { record("m3:boot"); return nil },
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// All boots before any post-boot.
	lastBoot := -1
	firstPostBoot := len(events)
	for i, e := range events {
		if strings.HasSuffix(e, ":boot") && i > lastBoot {
			lastBoot = i
		}
		if strings.HasSuffix(e, ":post-boot") && i < firstPostBoot {
			firstPostBoot = i
		}
	}
	if lastBoot >= firstPostBoot {
		t.Errorf("post-boot should come after all boots: events = %v", events)
	}
}

func TestPostBootFailureRollback(t *testing.T) {
	a := newTestApp(t)

	m1 := &testModule{BaseModule: BaseModule{ModuleName: "m1"}}
	m2 := &testModuleWithPostBoot{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "m2"}},
		postBootFn: func() error { return errors.New("m2 post-boot failed") },
	}
	m3 := &testModule{BaseModule: BaseModule{ModuleName: "m3"}}

	a.Use(m1, m2, m3)

	err := a.run()
	if err == nil || !strings.Contains(err.Error(), "m2 post-boot failed") {
		t.Fatalf("expected post-boot error, got: %v", err)
	}

	// All three modules booted → all three should be shut down on rollback.
	if !contains(m1.calls, "shutdown") {
		t.Error("m1 should have been shut down")
	}
	if !contains(m2.calls, "shutdown") {
		t.Error("m2 should have been shut down")
	}
	if !contains(m3.calls, "shutdown") {
		t.Error("m3 should have been shut down")
	}
}

func TestPreShutdown(t *testing.T) {
	a := newTestApp(t)

	var mu sync.Mutex
	var events []string
	record := func(s string) {
		mu.Lock()
		events = append(events, s)
		mu.Unlock()
	}

	m := &testModuleWithPreShutdown{
		testModule: testModule{
			BaseModule: BaseModule{ModuleName: "m1"},
			shutdownFn: func(_ context.Context) error { record("m1:shutdown"); return nil },
		},
		preShutdownFn: func(_ context.Context) error { record("m1:pre-shutdown"); return nil },
	}
	a.Use(m)

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Pre-shutdown should come before shutdown.
	preIdx := -1
	shutIdx := -1
	for i, e := range events {
		if e == "m1:pre-shutdown" {
			preIdx = i
		}
		if e == "m1:shutdown" {
			shutIdx = i
		}
	}
	if preIdx == -1 {
		t.Fatal("pre-shutdown not called")
	}
	if shutIdx == -1 {
		t.Fatal("shutdown not called")
	}
	if preIdx >= shutIdx {
		t.Errorf("pre-shutdown (idx=%d) should come before shutdown (idx=%d)", preIdx, shutIdx)
	}
}

func TestPreShutdownBestEffort(t *testing.T) {
	a := newTestApp(t)

	m1 := &testModuleWithPreShutdown{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "m1"}},
		preShutdownFn: func(_ context.Context) error {
			return errors.New("m1 pre-shutdown failed")
		},
	}
	m2 := &testModule{BaseModule: BaseModule{ModuleName: "m2"}}

	a.Use(m1, m2)

	err := a.run()
	// PreShutdown error should be in the returned error.
	if err == nil || !strings.Contains(err.Error(), "m1 pre-shutdown failed") {
		t.Fatalf("expected pre-shutdown error, got: %v", err)
	}

	// Both modules should still be shut down despite pre-shutdown error.
	if !contains(m1.calls, "shutdown") {
		t.Error("m1 should have been shut down")
	}
	if !contains(m2.calls, "shutdown") {
		t.Error("m2 should have been shut down")
	}
}

func TestFullLifecycleOrder(t *testing.T) {
	a := newTestApp(t)

	var mu sync.Mutex
	var events []string
	record := func(s string) {
		mu.Lock()
		events = append(events, s)
		mu.Unlock()
	}

	m := &testModuleWithBothHooks{
		testModule: testModule{
			BaseModule: BaseModule{ModuleName: "m1"},
			registerFn: func() error { record("register"); return nil },
			bootFn:     func() error { record("boot"); return nil },
			shutdownFn: func(_ context.Context) error { record("shutdown"); return nil },
		},
		postBootFn:    func() error { record("post-boot"); return nil },
		preShutdownFn: func(_ context.Context) error { record("pre-shutdown"); return nil },
	}
	a.Use(m)

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	want := []string{"register", "boot", "post-boot", "pre-shutdown", "shutdown"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Errorf("events[%d] = %q, want %q", i, events[i], want[i])
		}
	}
}

func TestModuleGroupPostBoot(t *testing.T) {
	var order []string
	record := func(s string) { order = append(order, s) }

	g := &ModuleGroup{
		GroupName: "admin",
		Modules: []Module{
			&testModule{
				BaseModule: BaseModule{ModuleName: "auth"},
				bootFn:     func() error { record("auth:boot"); return nil },
			},
			&testModuleWithPostBoot{
				testModule: testModule{
					BaseModule: BaseModule{ModuleName: "users"},
					bootFn:     func() error { record("users:boot"); return nil },
				},
				postBootFn: func() error { record("users:post-boot"); return nil },
			},
		},
	}

	if err := g.Register(); err != nil {
		t.Fatal(err)
	}
	if err := g.Boot(); err != nil {
		t.Fatal(err)
	}
	if err := g.PostBoot(); err != nil {
		t.Fatal(err)
	}

	want := []string{"auth:boot", "users:boot", "users:post-boot"}
	if len(order) != len(want) {
		t.Fatalf("events = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("events[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestModuleGroupPreShutdown(t *testing.T) {
	var order []string
	record := func(s string) { order = append(order, s) }

	g := &ModuleGroup{
		GroupName: "admin",
		Modules: []Module{
			&testModuleWithPreShutdown{
				testModule: testModule{
					BaseModule: BaseModule{ModuleName: "auth"},
					shutdownFn: func(_ context.Context) error { record("auth:shutdown"); return nil },
				},
				preShutdownFn: func(_ context.Context) error { record("auth:pre-shutdown"); return nil },
			},
			&testModule{
				BaseModule: BaseModule{ModuleName: "users"},
				shutdownFn: func(_ context.Context) error { record("users:shutdown"); return nil },
			},
		},
	}

	if err := g.Register(); err != nil {
		t.Fatal(err)
	}
	if err := g.Boot(); err != nil {
		t.Fatal(err)
	}

	// Pre-shutdown: only auth has PreShutdowner.
	if err := g.PreShutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Then shutdown in reverse.
	if err := g.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []string{"auth:pre-shutdown", "users:shutdown", "auth:shutdown"}
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

// ---------------------------------------------------------------------------
// Tagger / DependencyDeclarer / When
// ---------------------------------------------------------------------------

type testModuleWithTags struct {
	testModule
	tags []string
}

func (m *testModuleWithTags) Tags() []string { return m.tags }

type testModuleWithDeps struct {
	testModule
	deps []string
}

func (m *testModuleWithDeps) DependsOn() []string { return m.deps }

type testModuleWithTagsAndDeps struct {
	testModule
	tags []string
	deps []string
}

func (m *testModuleWithTagsAndDeps) Tags() []string      { return m.tags }
func (m *testModuleWithTagsAndDeps) DependsOn() []string  { return m.deps }

func TestDuplicateModuleNames(t *testing.T) {
	a := newTestApp(t)
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "dup"}})
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "dup"}})

	err := a.run()
	if err == nil || !strings.Contains(err.Error(), "duplicate module name") {
		t.Fatalf("expected duplicate name error, got: %v", err)
	}
}

func TestDependencyValidationPasses(t *testing.T) {
	a := newTestApp(t)
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "auth"}})
	a.Use(&testModuleWithDeps{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "users"}},
		deps:       []string{"auth"},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestDependencyValidationMissingDep(t *testing.T) {
	a := newTestApp(t)
	a.Use(&testModuleWithDeps{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "users"}},
		deps:       []string{"auth"},
	})

	err := a.run()
	if err == nil || !strings.Contains(err.Error(), `depends on "auth"`) {
		t.Fatalf("expected dependency error, got: %v", err)
	}
}

func TestDependencyValidationDisabledDep(t *testing.T) {
	a := newTestApp(t)
	a.Use(When(false, &testModule{BaseModule: BaseModule{ModuleName: "auth"}}))
	a.Use(&testModuleWithDeps{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "users"}},
		deps:       []string{"auth"},
	})

	err := a.run()
	if err == nil || !strings.Contains(err.Error(), `depends on "auth"`) {
		t.Fatalf("expected dependency error for disabled dep, got: %v", err)
	}
}

func TestDependencyValidationMultipleDeps(t *testing.T) {
	a := newTestApp(t)
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "auth"}})
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "notifications"}})
	a.Use(&testModuleWithDeps{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "users"}},
		deps:       []string{"auth", "notifications"},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestWhenTrue(t *testing.T) {
	a := newTestApp(t)
	m := &testModule{BaseModule: BaseModule{ModuleName: "feature"}}
	a.Use(When(true, m))

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if !contains(m.calls, "register") || !contains(m.calls, "boot") {
		t.Errorf("When(true) should run lifecycle, calls = %v", m.calls)
	}
}

func TestWhenFalse(t *testing.T) {
	a := newTestApp(t)
	m := &testModule{BaseModule: BaseModule{ModuleName: "feature"}}
	a.Use(When(false, m))

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if contains(m.calls, "register") || contains(m.calls, "boot") || contains(m.calls, "shutdown") {
		t.Errorf("When(false) should skip all lifecycle, calls = %v", m.calls)
	}
}

func TestWhenFalseSkipsPostBoot(t *testing.T) {
	a := newTestApp(t)
	m := &testModuleWithPostBoot{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "feature"}},
	}
	a.Use(When(false, m))

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if contains(m.calls, "post-boot") {
		t.Error("disabled module should not run PostBoot")
	}
}

func TestWhenFalsePreservesName(t *testing.T) {
	m := When(false, &testModule{BaseModule: BaseModule{ModuleName: "email"}})
	if m.Name() != "email" {
		t.Errorf("Name() = %q, want %q", m.Name(), "email")
	}
}

func TestWhenMixedModules(t *testing.T) {
	a := newTestApp(t)

	active := &testModule{BaseModule: BaseModule{ModuleName: "active"}}
	disabled := &testModule{BaseModule: BaseModule{ModuleName: "disabled"}}

	a.Use(active)
	a.Use(When(false, disabled))

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Active module ran full lifecycle.
	if !contains(active.calls, "register") || !contains(active.calls, "boot") {
		t.Errorf("active module should run lifecycle, calls = %v", active.calls)
	}
	// Disabled module skipped entirely.
	if contains(disabled.calls, "register") || contains(disabled.calls, "boot") {
		t.Errorf("disabled module should skip lifecycle, calls = %v", disabled.calls)
	}
}

func TestModuleInfoBasic(t *testing.T) {
	a := newTestApp(t)
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "auth"}})
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "users"}})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	infos := a.ModuleInfo()
	if len(infos) != 2 {
		t.Fatalf("ModuleInfo() returned %d entries, want 2", len(infos))
	}

	// After run() returns, ready=false and booted is populated → status=shutdown.
	if infos[0].Name != "auth" || infos[0].Status != ModuleStatusShutdown {
		t.Errorf("infos[0] = {%q, %q}, want {auth, shutdown}", infos[0].Name, infos[0].Status)
	}
	if infos[1].Name != "users" || infos[1].Status != ModuleStatusShutdown {
		t.Errorf("infos[1] = {%q, %q}, want {users, shutdown}", infos[1].Name, infos[1].Status)
	}
}

func TestModuleInfoWithTags(t *testing.T) {
	a := newTestApp(t)
	a.Use(&testModuleWithTags{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "auth"}},
		tags:       []string{"builtin", "security"},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	infos := a.ModuleInfo()
	if len(infos[0].Tags) != 2 || infos[0].Tags[0] != "builtin" || infos[0].Tags[1] != "security" {
		t.Errorf("infos[0].Tags = %v, want [builtin security]", infos[0].Tags)
	}
}

func TestModuleInfoWithDeps(t *testing.T) {
	a := newTestApp(t)
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "auth"}})
	a.Use(&testModuleWithDeps{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "users"}},
		deps:       []string{"auth"},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	infos := a.ModuleInfo()
	if len(infos[1].Dependencies) != 1 || infos[1].Dependencies[0] != "auth" {
		t.Errorf("infos[1].Dependencies = %v, want [auth]", infos[1].Dependencies)
	}
}

func TestModuleInfoDisabledPreservesMetadata(t *testing.T) {
	a := newTestApp(t)
	a.Use(When(false, &testModuleWithTagsAndDeps{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "email"}},
		tags:       []string{"optional"},
		deps:       []string{"auth"},
	}))

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	infos := a.ModuleInfo()
	if len(infos) != 1 {
		t.Fatalf("ModuleInfo() returned %d entries, want 1", len(infos))
	}
	if infos[0].Status != ModuleStatusDisabled {
		t.Errorf("Status = %q, want %q", infos[0].Status, ModuleStatusDisabled)
	}
	if len(infos[0].Tags) != 1 || infos[0].Tags[0] != "optional" {
		t.Errorf("Tags = %v, want [optional]", infos[0].Tags)
	}
	if len(infos[0].Dependencies) != 1 || infos[0].Dependencies[0] != "auth" {
		t.Errorf("Dependencies = %v, want [auth]", infos[0].Dependencies)
	}
}

func TestModuleInfoStatusDuringBoot(t *testing.T) {
	a := newTestApp(t)

	var capturedInfo []ModuleInfo
	a.Use(&testModule{
		BaseModule: BaseModule{ModuleName: "probe"},
		bootFn: func() error {
			capturedInfo = a.ModuleInfo()
			return nil
		},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// During boot, ready=false, module is not yet in booted set → registered.
	if len(capturedInfo) != 1 {
		t.Fatalf("captured %d entries, want 1", len(capturedInfo))
	}
	if capturedInfo[0].Status != ModuleStatusRegistered {
		t.Errorf("status during boot = %q, want %q", capturedInfo[0].Status, ModuleStatusRegistered)
	}
}

func TestModuleNamesShowsDisabled(t *testing.T) {
	a := newTestApp(t)
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "auth"}})
	a.Use(When(false, &testModule{BaseModule: BaseModule{ModuleName: "email"}}))

	names := a.moduleNames()
	if len(names) != 2 {
		t.Fatalf("moduleNames() returned %d, want 2", len(names))
	}
	if names[0] != "auth" {
		t.Errorf("names[0] = %q, want %q", names[0], "auth")
	}
	if names[1] != "email (disabled)" {
		t.Errorf("names[1] = %q, want %q", names[1], "email (disabled)")
	}
}

func TestDisabledModulesNotInDependencyValidation(t *testing.T) {
	// A disabled module with DependsOn should NOT be validated.
	a := newTestApp(t)
	a.Use(When(false, &testModuleWithDeps{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "email"}},
		deps:       []string{"nonexistent"},
	}))

	if err := a.run(); err != nil {
		t.Fatalf("disabled module's deps should not be validated, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ModuleInfo children introspection
// ---------------------------------------------------------------------------

func TestModuleInfoGroupChildren(t *testing.T) {
	a := newTestApp(t)
	a.Use(&ModuleGroup{
		GroupName: "admin",
		Modules: []Module{
			&testModule{BaseModule: BaseModule{ModuleName: "auth"}},
			&testModule{BaseModule: BaseModule{ModuleName: "users"}},
		},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	infos := a.ModuleInfo()
	if len(infos) != 1 {
		t.Fatalf("ModuleInfo() returned %d entries, want 1", len(infos))
	}
	if infos[0].Name != "admin" {
		t.Errorf("Name = %q, want %q", infos[0].Name, "admin")
	}
	if len(infos[0].Children) != 2 {
		t.Fatalf("Children count = %d, want 2", len(infos[0].Children))
	}
	if infos[0].Children[0].Name != "auth" || infos[0].Children[1].Name != "users" {
		t.Errorf("Children names = [%q, %q], want [auth, users]",
			infos[0].Children[0].Name, infos[0].Children[1].Name)
	}
	// After run(), status is shutdown (ready=false, but booted).
	if infos[0].Children[0].Status != ModuleStatusShutdown {
		t.Errorf("child status = %q, want %q", infos[0].Children[0].Status, ModuleStatusShutdown)
	}
}

func TestModuleInfoGroupChildrenBooted(t *testing.T) {
	// Capture ModuleInfo during running phase (inside PostBoot).
	a := newTestApp(t)

	var capturedInfo []ModuleInfo
	a.Use(&ModuleGroup{
		GroupName: "admin",
		Modules: []Module{
			&testModuleWithPostBoot{
				testModule: testModule{BaseModule: BaseModule{ModuleName: "auth"}},
				postBootFn: func() error {
					capturedInfo = a.ModuleInfo()
					return nil
				},
			},
			&testModule{BaseModule: BaseModule{ModuleName: "users"}},
		},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// During PostBoot, ready=false, but children are in group's booted set.
	// Group itself is in app's booted set. Since ready=false → shutdown status.
	// But children are in group's booted → they use group's bootedSet, appReady=false → shutdown.
	// Actually during postBoot, ready=false and modules are booted → status is shutdown.
	// This is the transient state — the important test is the Children exist.
	if len(capturedInfo) != 1 {
		t.Fatalf("captured %d entries, want 1", len(capturedInfo))
	}
	if len(capturedInfo[0].Children) != 2 {
		t.Fatalf("Children count = %d, want 2", len(capturedInfo[0].Children))
	}
}

func TestModuleInfoDisabledGroupPreservesChildren(t *testing.T) {
	a := newTestApp(t)
	a.Use(When(false, &ModuleGroup{
		GroupName: "admin",
		Modules: []Module{
			&testModuleWithTags{
				testModule: testModule{BaseModule: BaseModule{ModuleName: "auth"}},
				tags:       []string{"builtin"},
			},
			&testModule{BaseModule: BaseModule{ModuleName: "users"}},
		},
	}))

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	infos := a.ModuleInfo()
	if len(infos) != 1 {
		t.Fatalf("ModuleInfo() returned %d entries, want 1", len(infos))
	}
	if infos[0].Status != ModuleStatusDisabled {
		t.Errorf("group Status = %q, want %q", infos[0].Status, ModuleStatusDisabled)
	}
	if len(infos[0].Children) != 2 {
		t.Fatalf("Children count = %d, want 2", len(infos[0].Children))
	}
	// Children of disabled group inherit disabled status.
	if infos[0].Children[0].Status != ModuleStatusDisabled {
		t.Errorf("child Status = %q, want %q", infos[0].Children[0].Status, ModuleStatusDisabled)
	}
	// Child metadata preserved even though group is disabled.
	if len(infos[0].Children[0].Tags) != 1 || infos[0].Children[0].Tags[0] != "builtin" {
		t.Errorf("child Tags = %v, want [builtin]", infos[0].Children[0].Tags)
	}
}

func TestModuleInfoNestedGroups(t *testing.T) {
	a := newTestApp(t)
	a.Use(&ModuleGroup{
		GroupName: "admin",
		Modules: []Module{
			&testModule{BaseModule: BaseModule{ModuleName: "auth"}},
			&ModuleGroup{
				GroupName: "management",
				Modules: []Module{
					&testModule{BaseModule: BaseModule{ModuleName: "user-mgmt"}},
					&testModule{BaseModule: BaseModule{ModuleName: "log-mgmt"}},
				},
			},
		},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	infos := a.ModuleInfo()
	if len(infos) != 1 || infos[0].Name != "admin" {
		t.Fatalf("top-level = %v, want [admin]", infos)
	}
	if len(infos[0].Children) != 2 {
		t.Fatalf("admin children = %d, want 2", len(infos[0].Children))
	}

	mgmt := infos[0].Children[1]
	if mgmt.Name != "management" {
		t.Errorf("nested group name = %q, want %q", mgmt.Name, "management")
	}
	if len(mgmt.Children) != 2 {
		t.Fatalf("management children = %d, want 2", len(mgmt.Children))
	}
	if mgmt.Children[0].Name != "user-mgmt" || mgmt.Children[1].Name != "log-mgmt" {
		t.Errorf("nested children = [%q, %q], want [user-mgmt, log-mgmt]",
			mgmt.Children[0].Name, mgmt.Children[1].Name)
	}
}

// ---------------------------------------------------------------------------
// Boot-order validation
// ---------------------------------------------------------------------------

func TestDependencyValidationBootOrder(t *testing.T) {
	// Module "users" depends on "auth", but "users" is registered first.
	a := newTestApp(t)
	a.Use(&testModuleWithDeps{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "users"}},
		deps:       []string{"auth"},
	})
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "auth"}})

	err := a.run()
	if err == nil {
		t.Fatal("expected boot-order error, got nil")
	}
	if !strings.Contains(err.Error(), "registered after") {
		t.Errorf("error = %q, want substring 'registered after'", err.Error())
	}
}

func TestDependencyValidationBootOrderCorrect(t *testing.T) {
	// Correct order: auth before users.
	a := newTestApp(t)
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "auth"}})
	a.Use(&testModuleWithDeps{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "users"}},
		deps:       []string{"auth"},
	})

	if err := a.run(); err != nil {
		t.Fatalf("correct order should succeed: %v", err)
	}
}

func TestDependencyValidationBootOrderMultiple(t *testing.T) {
	// Module "dashboard" depends on "auth" and "db". "db" is registered after dashboard.
	a := newTestApp(t)
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "auth"}})
	a.Use(&testModuleWithDeps{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "dashboard"}},
		deps:       []string{"auth", "db"},
	})
	a.Use(&testModule{BaseModule: BaseModule{ModuleName: "db"}})

	err := a.run()
	if err == nil {
		t.Fatal("expected boot-order error for 'db', got nil")
	}
	if !strings.Contains(err.Error(), "db") || !strings.Contains(err.Error(), "registered after") {
		t.Errorf("error = %q, want mention of 'db' and 'registered after'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// HealthCheckProvider
// ---------------------------------------------------------------------------

type testModuleWithHealthChecks struct {
	testModule
	checkers []HealthChecker
}

func (m *testModuleWithHealthChecks) HealthChecks() []HealthChecker {
	return m.checkers
}

func TestHealthCheckProviderAutoRegistered(t *testing.T) {
	a := newTestApp(t)

	pingCalled := false
	a.Use(&testModuleWithHealthChecks{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "cache"}},
		checkers: []HealthChecker{
			CheckFunc{CheckerName: "cache-ping", Fn: func(_ context.Context) error {
				pingCalled = true
				return nil
			}},
		},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	report := a.CheckHealth(context.Background())
	if !pingCalled {
		t.Fatal("health check from HealthCheckProvider was not called")
	}

	// Should have sqlite (framework) + cache-ping (module).
	found := false
	for _, c := range report.Components {
		if c.Name == "cache-ping" {
			found = true
			if !c.Healthy {
				t.Errorf("cache-ping unhealthy: %s", c.Error)
			}
		}
	}
	if !found {
		t.Errorf("cache-ping not found in health report components: %v", report.Components)
	}
}

func TestHealthCheckProviderInGroup(t *testing.T) {
	a := newTestApp(t)

	a.Use(&ModuleGroup{
		GroupName: "infra",
		Modules: []Module{
			&testModuleWithHealthChecks{
				testModule: testModule{BaseModule: BaseModule{ModuleName: "queue"}},
				checkers: []HealthChecker{
					CheckFunc{CheckerName: "queue-depth", Fn: func(_ context.Context) error {
						return nil
					}},
				},
			},
		},
	})

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	report := a.CheckHealth(context.Background())
	found := false
	for _, c := range report.Components {
		if c.Name == "queue-depth" {
			found = true
		}
	}
	if !found {
		t.Errorf("queue-depth not found in health report; group children should auto-register health checks")
	}
}

func TestHealthCheckProviderDisabledModuleSkipped(t *testing.T) {
	a := newTestApp(t)

	a.Use(When(false, &testModuleWithHealthChecks{
		testModule: testModule{BaseModule: BaseModule{ModuleName: "cache"}},
		checkers: []HealthChecker{
			CheckFunc{CheckerName: "should-not-exist", Fn: func(_ context.Context) error {
				return nil
			}},
		},
	}))

	if err := a.run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	report := a.CheckHealth(context.Background())
	for _, c := range report.Components {
		if c.Name == "should-not-exist" {
			t.Fatal("disabled module's health checks should not be registered")
		}
	}
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
