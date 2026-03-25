package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
	sunkerndb "sunkern.local/framework/sqlite"
	"sunkern.local/framework/sqlite/migrate"
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
	readyCh chan struct{} // closed when app is ready
	runOnce sync.Once    // ensures Run is called exactly once

	// Health checks registered by framework services and modules.
	healthCheckers []HealthChecker
	healthMu       sync.RWMutex

	// For testing: override signal context creation.
	signalCtxFunc func() (context.Context, context.CancelFunc)

	shutdownTimeout  time.Duration
	migrationSources []fs.FS
	version          string
	env              string // resolved from config during Run
}

// New creates an application. Options are applied in order.
func New(opts ...Option) *App {
	a := &App{
		shutdownTimeout: 30 * time.Second,
	}
	a.shutdown.ch = make(chan struct{})
	a.readyCh = make(chan struct{})
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

// WithMigrations adds migration sources to the application. Each source is
// an fs.FS containing *.sql migration files. Framework and service migrations
// are merged and applied in version order during boot.
func WithMigrations(sources ...fs.FS) Option {
	return func(a *App) { a.migrationSources = append(a.migrationSources, sources...) }
}

// WithVersion sets the application version string. Included in the startup
// log and available via App.Version(). Typically set from build-time ldflags
// or a constant in main.go.
func WithVersion(v string) Option {
	return func(a *App) { a.version = v }
}

// ModuleStatus represents a module's position in the application lifecycle.
type ModuleStatus string

const (
	ModuleStatusRegistered ModuleStatus = "registered"
	ModuleStatusBooted     ModuleStatus = "booted"
	ModuleStatusShutdown   ModuleStatus = "shutdown"
	ModuleStatusDisabled   ModuleStatus = "disabled"
)

// ModuleInfo describes a module for admin introspection. Serialize
// directly to JSON for admin API responses. For ModuleGroup modules,
// Children contains recursive info about each child module.
type ModuleInfo struct {
	Name         string       `json:"name"`
	Status       ModuleStatus `json:"status"`
	Tags         []string     `json:"tags,omitempty"`
	Dependencies []string     `json:"dependencies,omitempty"`
	Children     []ModuleInfo `json:"children,omitempty"`
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

// ReadyCh returns a channel that is closed when the application finishes
// booting and begins accepting traffic. Use it in background goroutines
// that should wait for full readiness before starting work:
//
//	go func() {
//	    <-app.ReadyCh()
//	    // all modules booted, safe to start
//	}()
func (a *App) ReadyCh() <-chan struct{} {
	return a.readyCh
}

// Version returns the application version string set via WithVersion.
// Returns empty string if not set.
func (a *App) Version() string {
	return a.version
}

// Env returns the application environment ("development", "production",
// "test", etc.). Resolved from the "app.env" config key (APP_ENV env var)
// during Run, defaulting to "development".
func (a *App) Env() string {
	return a.env
}

// ModuleInfo returns metadata about all registered modules, including
// disabled ones. Status reflects the current lifecycle position.
// ModuleGroup modules include recursive Children info. Safe to call
// from HTTP handlers during the running phase.
func (a *App) ModuleInfo() []ModuleInfo {
	bootedSet := make(map[string]bool, len(a.booted))
	for _, m := range a.booted {
		bootedSet[m.Name()] = true
	}

	appReady := a.ready.Load()
	infos := make([]ModuleInfo, len(a.modules))
	for i, m := range a.modules {
		infos[i] = buildModuleInfo(m, bootedSet, appReady, false)
	}
	return infos
}

// buildModuleInfo constructs a ModuleInfo for a single module, recursively
// expanding ModuleGroup children. parentDisabled propagates the disabled
// state from a disabled group to all its children.
func buildModuleInfo(m Module, bootedSet map[string]bool, appReady bool, parentDisabled bool) ModuleInfo {
	// Unwrap disabled wrapper to access inner module metadata.
	target := m
	disabled := parentDisabled || isDisabled(m)
	if d, ok := m.(*disabledModule); ok {
		target = d.inner
	}

	// Determine status.
	var status ModuleStatus
	switch {
	case disabled:
		status = ModuleStatusDisabled
	case !bootedSet[m.Name()]:
		status = ModuleStatusRegistered
	case appReady:
		status = ModuleStatusBooted
	default:
		status = ModuleStatusShutdown
	}

	info := ModuleInfo{
		Name:   m.Name(),
		Status: status,
	}

	// Extract optional metadata from target (inner module for disabled).
	if t, ok := target.(Tagger); ok {
		info.Tags = t.Tags()
	}
	if dd, ok := target.(DependencyDeclarer); ok {
		info.Dependencies = dd.DependsOn()
	}

	// Expand ModuleGroup children recursively.
	if g, ok := target.(*ModuleGroup); ok {
		childBootedSet := make(map[string]bool, len(g.booted))
		for _, c := range g.booted {
			childBootedSet[c.Name()] = true
		}
		children := make([]ModuleInfo, len(g.Modules))
		for i, child := range g.Modules {
			children[i] = buildModuleInfo(child, childBootedSet, appReady && !disabled, disabled)
		}
		info.Children = children
	}

	return info
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

	bootStart := time.Now()

	// Load config from file + env vars. The global container is already
	// initialized by its package init — no Reset() needed here. Reset()
	// exists solely for tests that need a clean container between cases.
	config.Load()

	// Register app-level config defaults for discoverability via
	// config.Keys() and config.All(). Must be after Load() which resets
	// all defaults.
	config.SetDefault("app.env", "development")

	// Resolve environment after config loads.
	a.env = config.GetOr[string]("app.env", "development")

	// Supply the App itself so modules can resolve it from the container
	// to access Ready(), ShuttingDown(), ReadyCh(), CheckHealth() without
	// explicit passing.
	container.Supply(a)

	sunkernlog.Load()

	// Phase 1: Register all modules. Every module's Register() runs before
	// any module's Boot(). Only container.Provide / config.SetDefault calls
	// belong in Register.
	phaseStart := time.Now()
	slog.Info("registering modules", "count", len(a.modules), "modules", a.moduleNames())
	if err := a.register(); err != nil {
		return err
	}
	registerMs := time.Since(phaseStart).Milliseconds()

	// Validate declared module dependencies. Catches misconfigured
	// module ordering before any framework service initialization.
	if err := a.validateDependencies(); err != nil {
		return err
	}

	// Phase 2: Init framework services.
	phaseStart = time.Now()
	t := time.Now()
	sunkerndb.Load()
	container.Supply[*sunkerndb.DB](sunkerndb.Global())
	a.AddHealthCheck(CheckFunc{CheckerName: "sqlite", Fn: sunkerndb.Global().Health})
	slog.Debug("framework service initialized", "service", "sqlite", "took", time.Since(t))

	// Run migrations.
	t = time.Now()
	migrationEngine := migrate.NewEngine(sunkerndb.Global().WriteDB())
	if err := migrationEngine.Collect(a.migrationSources...); err != nil {
		return fmt.Errorf("collect migrations: %w", err)
	}
	if n, err := migrationEngine.Up(context.Background()); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	} else if n > 0 {
		slog.Info("migrations applied", "count", n, "took", time.Since(t))
	}

	// Validate migration integrity — warn about orphaned DB records
	// (applied but no file) and dirty checksums (file changed after apply).
	if vr, err := migrationEngine.Validate(context.Background()); err != nil {
		slog.Warn("migration validation failed", "error", err)
	} else if !vr.Clean() {
		for _, o := range vr.Orphaned {
			slog.Warn("orphaned migration: applied in database but no file found",
				"version", o.Version,
				"name", o.Name,
				"applied_at", o.AppliedAt,
			)
		}
		for _, d := range vr.Dirty {
			slog.Warn("dirty migration: file changed after apply",
				"version", d.Version,
				"name", d.Name,
				"file_checksum", d.FileChecksum,
				"db_checksum", d.DBChecksum,
			)
		}
	}
	container.Supply(migrationEngine)

	// TODO: init cache, event bus, cron, queue.
	t = time.Now()
	srv, err := sunkernhttp.NewServer()
	if err != nil {
		return fmt.Errorf("init http: %w", err)
	}
	container.Supply[sunkernhttp.Server](srv)
	slog.Debug("framework service initialized", "service", "http", "took", time.Since(t))

	// Validate all registered config rules. This runs after framework
	// services (which register their own defaults and rules) and after
	// module Register (which registers service-level defaults and rules).
	// Catches misconfigurations before any module's Boot phase.
	config.Validate()
	frameworkMs := time.Since(phaseStart).Milliseconds()

	// Phase 3: Boot all modules. Modules resolve services from the container
	// and wire routes, event handlers, cron jobs, etc.
	phaseStart = time.Now()
	if err := a.boot(); err != nil {
		return err
	}
	moduleBootMs := time.Since(phaseStart).Milliseconds()

	// Phase 3b: Post-boot. Modules implementing PostBooter run setup after
	// ALL modules have completed Boot — data seeding, cache warm-up, etc.
	phaseStart = time.Now()
	if err := a.postBoot(); err != nil {
		return err
	}
	postBootMs := time.Since(phaseStart).Milliseconds()

	// Phase 4: Start lifecycle hooks.
	phaseStart = time.Now()
	ctx := context.Background()
	hookReports, err := container.Global().StartHooks(ctx)
	if err != nil {
		_ = a.shutdownModules(context.Background())
		return fmt.Errorf("start hooks: %w", err)
	}
	for _, r := range hookReports {
		slog.Debug("hook started", "hook", r.Name, "took_ms", r.DurationMs)
	}
	hooksMs := time.Since(phaseStart).Milliseconds()

	a.ready.Store(true)
	close(a.readyCh)

	startAttrs := []any{
		"env", a.env,
		"boot_time_ms", time.Since(bootStart).Milliseconds(),
		"register_ms", registerMs,
		"framework_ms", frameworkMs,
		"module_boot_ms", moduleBootMs,
		"post_boot_ms", postBootMs,
		"hooks_ms", hooksMs,
		"modules", len(a.modules),
		"services", container.Len(),
		"hooks", len(container.Global().Hooks()),
		"health_checks", len(a.healthCheckers),
	}
	if a.version != "" {
		startAttrs = append(startAttrs, "version", a.version)
	}
	slog.Info("application started", startAttrs...)

	// Phase 5: Wait for shutdown signal.
	a.waitForSignal()

	// Phase 6: Graceful shutdown.
	shutdownStart := time.Now()
	slog.Info("shutting down")
	a.shutdown.once.Do(func() { close(a.shutdown.ch) })
	a.ready.Store(false)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()

	var errs []error

	// Pre-shutdown: modules implementing PreShutdowner prepare for shutdown
	// (stop accepting work, drain in-flight requests, flush buffers).
	if err := a.preShutdown(shutdownCtx); err != nil {
		errs = append(errs, err)
	}

	stopReports, stopErr := container.Global().StopHooks(shutdownCtx)
	if stopErr != nil {
		errs = append(errs, stopErr)
	}
	for _, r := range stopReports {
		attrs := []any{"hook", r.Name, "took_ms", r.DurationMs}
		if r.Err != "" {
			attrs = append(attrs, "error", r.Err)
		}
		slog.Debug("hook stopped", attrs...)
	}
	if err := a.shutdownModules(shutdownCtx); err != nil {
		errs = append(errs, err)
	}

	slog.Info("shutdown complete", "took", time.Since(shutdownStart))
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (a *App) register() error {
	// Validate: no duplicate module names.
	seen := make(map[string]bool, len(a.modules))
	for _, m := range a.modules {
		name := m.Name()
		if seen[name] {
			return fmt.Errorf("duplicate module name: %q", name)
		}
		seen[name] = true
	}

	for _, m := range a.modules {
		if isDisabled(m) {
			slog.Debug("module disabled", "module", m.Name())
			continue
		}
		slog.Debug("registering module", "module", m.Name())
		if err := m.Register(); err != nil {
			return fmt.Errorf("register %q: %w", m.Name(), err)
		}
	}
	return nil
}

func (a *App) boot() error {
	for _, m := range a.modules {
		if isDisabled(m) {
			continue
		}
		t := time.Now()
		if err := m.Boot(); err != nil {
			// Rollback: shutdown modules that already booted.
			_ = a.shutdownModules(context.Background())
			return fmt.Errorf("boot %q: %w", m.Name(), err)
		}
		slog.Debug("module booted", "module", m.Name(), "took", time.Since(t))
		a.booted = append(a.booted, m)

		// Auto-register health checks from modules that provide them.
		a.registerModuleHealthChecks(m)
	}
	return nil
}

// registerModuleHealthChecks checks if a module (or its children, for
// ModuleGroups) implements HealthCheckProvider and registers all
// returned health checkers.
func (a *App) registerModuleHealthChecks(m Module) {
	if hp, ok := m.(HealthCheckProvider); ok {
		for _, hc := range hp.HealthChecks() {
			a.AddHealthCheck(hc)
		}
	}
	// For ModuleGroups, also check children (they boot through the group).
	if g, ok := m.(*ModuleGroup); ok {
		for _, child := range g.booted {
			a.registerModuleHealthChecks(child)
		}
	}
}

func (a *App) postBoot() error {
	for _, m := range a.booted {
		pb, ok := m.(PostBooter)
		if !ok {
			continue
		}
		t := time.Now()
		if err := pb.PostBoot(); err != nil {
			_ = a.shutdownModules(context.Background())
			return fmt.Errorf("post-boot %q: %w", m.Name(), err)
		}
		slog.Debug("module post-booted", "module", m.Name(), "took", time.Since(t))
	}
	return nil
}

func (a *App) preShutdown(ctx context.Context) error {
	var errs []error
	for _, m := range a.booted {
		ps, ok := m.(PreShutdowner)
		if !ok {
			continue
		}
		t := time.Now()
		if err := ps.PreShutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("pre-shutdown %q: %w", m.Name(), err))
		}
		slog.Debug("module pre-shutdown complete", "module", m.Name(), "took", time.Since(t))
	}
	return errors.Join(errs...)
}

func (a *App) shutdownModules(ctx context.Context) error {
	var errs []error
	for i := len(a.booted) - 1; i >= 0; i-- {
		t := time.Now()
		if err := a.booted[i].Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown %q: %w", a.booted[i].Name(), err))
		}
		slog.Debug("module shut down", "module", a.booted[i].Name(), "took", time.Since(t))
	}
	return errors.Join(errs...)
}

func (a *App) validateDependencies() error {
	// Build position index: name → registration index. Earlier index = boots first.
	position := make(map[string]int, len(a.modules))
	for i, m := range a.modules {
		if !isDisabled(m) {
			position[m.Name()] = i
		}
	}
	for _, m := range a.modules {
		if isDisabled(m) {
			continue
		}
		dd, ok := m.(DependencyDeclarer)
		if !ok {
			continue
		}
		modPos := position[m.Name()]
		for _, dep := range dd.DependsOn() {
			depPos, exists := position[dep]
			if !exists {
				return fmt.Errorf("module %q depends on %q, which is not registered or is disabled", m.Name(), dep)
			}
			// Dependency must be registered before the dependent module
			// so it boots first. Registration order = boot order.
			if depPos > modPos {
				return fmt.Errorf("module %q depends on %q, but %q is registered after %q (dependencies must be registered first)", m.Name(), dep, dep, m.Name())
			}
		}
	}
	return nil
}

func (a *App) moduleNames() []string {
	names := make([]string, len(a.modules))
	for i, m := range a.modules {
		if isDisabled(m) {
			names[i] = m.Name() + " (disabled)"
		} else {
			names[i] = m.Name()
		}
	}
	return names
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
