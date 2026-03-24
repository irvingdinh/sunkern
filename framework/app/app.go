package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"sunkern.local/framework/config"
	"sunkern.local/framework/container"
	sunkernhttp "sunkern.local/framework/http"
	sunkernlog "sunkern.local/framework/log"
)

// App orchestrates the full lifecycle of a Sunkern application. It manages
// module registration, boot sequencing, signal handling, and graceful
// shutdown. Create one with New, register modules with Use, and block on
// Run.
type App struct {
	modules  []Module
	booted   []Module // modules that completed Boot (for rollback)
	shutdown struct {
		once sync.Once
		ch   chan struct{} // closed when shutdown begins
	}
	ready   atomic.Bool
	runOnce sync.Once // ensures Run is called exactly once

	// For testing: override signal context creation.
	signalCtxFunc func() (context.Context, context.CancelFunc)

	shutdownTimeout time.Duration
}

// New creates an application. Options are applied in order.
func New(opts ...Option) *App {
	a := &App{
		shutdownTimeout: 30 * time.Second,
	}
	a.shutdown.ch = make(chan struct{})
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Option configures an App.
type Option func(*App)

// WithShutdownTimeout sets the maximum time allowed for graceful shutdown.
// Default is 30 seconds.
func WithShutdownTimeout(d time.Duration) Option {
	return func(a *App) { a.shutdownTimeout = d }
}

// Use adds modules to the application. Modules are registered and booted
// in the order they are added.
func (a *App) Use(modules ...Module) {
	a.modules = append(a.modules, modules...)
}

// Ready reports whether the application has finished booting and is
// accepting traffic. Returns false before Run completes boot and after
// shutdown begins.
func (a *App) Ready() bool {
	return a.ready.Load()
}

// ShuttingDown returns a channel that is closed when graceful shutdown
// begins. Use it in long-running goroutines to detect shutdown:
//
//	select {
//	case <-app.ShuttingDown():
//	    return
//	case msg := <-ch:
//	    process(msg)
//	}
func (a *App) ShuttingDown() <-chan struct{} {
	return a.shutdown.ch
}

// Run executes the full application lifecycle. It blocks until a shutdown
// signal is received (SIGINT or SIGTERM), then gracefully shuts down. If
// any phase fails, Run logs the error and terminates the process — the
// caller never handles the error.
func (a *App) Run() {
	if err := a.run(); err != nil {
		slog.Error("fatal error", "error", err)
		sunkernlog.Close()
		os.Exit(1)
	}
}

// run is the internal implementation of Run. It returns an error so that
// tests (which live in the same package) can inspect failures without
// triggering log.Fatal.
func (a *App) run() error {
	// Run must be called exactly once. The global container and config are
	// singletons — calling Run twice would corrupt their state. Panic on
	// the second call to catch programming errors early.
	started := false
	a.runOnce.Do(func() { started = true })
	if !started {
		panic("app: Run called more than once")
	}

	// Load config from file + env vars. The global container is already
	// initialized by its package init — no Reset() needed here. Reset()
	// exists solely for tests that need a clean container between cases.
	config.Load()

	if err := sunkernlog.Init(); err != nil {
		return fmt.Errorf("init log: %w", err)
	}

	// Phase 1: Register all modules. Every module's Register() runs before
	// any module's Boot(). Only container.Provide / config.SetDefault calls
	// belong in Register.
	if err := a.register(); err != nil {
		return err
	}

	// Phase 2: Init framework services.
	// TODO: init DB, cache, event bus, cron, queue, run migrations.
	srv, err := sunkernhttp.NewServer()
	if err != nil {
		return fmt.Errorf("init http: %w", err)
	}
	container.Supply[sunkernhttp.Server](srv)

	// Phase 3: Boot all modules. Modules resolve services from the container
	// and wire routes, event handlers, cron jobs, etc.
	if err := a.boot(); err != nil {
		return err
	}

	// Phase 4: Start lifecycle hooks.
	ctx := context.Background()
	if err := container.Global().StartHooks(ctx); err != nil {
		_ = a.shutdownModules(context.Background())
		return fmt.Errorf("start hooks: %w", err)
	}

	a.ready.Store(true)
	slog.Info("application started", "log_type", "lifecycle", "addr", config.GetOr[string]("http.addr", sunkernhttp.DefaultAddr))

	// Phase 5: Wait for shutdown signal.
	a.waitForSignal()

	// Phase 6: Graceful shutdown.
	slog.Info("shutting down", "log_type", "lifecycle")
	a.shutdown.once.Do(func() { close(a.shutdown.ch) })
	a.ready.Store(false)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()

	var errs []error
	if err := container.Global().StopHooks(shutdownCtx); err != nil {
		errs = append(errs, err)
	}
	if err := a.shutdownModules(shutdownCtx); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (a *App) register() error {
	for _, m := range a.modules {
		if err := m.Register(); err != nil {
			return fmt.Errorf("register %q: %w", m.Name(), err)
		}
	}
	return nil
}

func (a *App) boot() error {
	for _, m := range a.modules {
		if err := m.Boot(); err != nil {
			// Rollback: shutdown modules that already booted.
			_ = a.shutdownModules(context.Background())
			return fmt.Errorf("boot %q: %w", m.Name(), err)
		}
		a.booted = append(a.booted, m)
	}
	return nil
}

func (a *App) shutdownModules(ctx context.Context) error {
	var errs []error
	for i := len(a.booted) - 1; i >= 0; i-- {
		if err := a.booted[i].Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown %q: %w", a.booted[i].Name(), err))
		}
	}
	return errors.Join(errs...)
}

func (a *App) waitForSignal() {
	var ctx context.Context
	var stop context.CancelFunc

	if a.signalCtxFunc != nil {
		ctx, stop = a.signalCtxFunc()
	} else {
		ctx, stop = signal.NotifyContext(context.Background(),
			os.Interrupt, syscall.SIGTERM)
	}

	<-ctx.Done()

	// Restore default signal behavior so a second SIGINT/SIGTERM terminates
	// the process immediately (double-signal escalation).
	stop()
}
