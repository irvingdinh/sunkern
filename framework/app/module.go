package app

import (
	"context"
	"errors"
	"fmt"
)

// Module is the unit of composition in a Sunkern application. Each module
// goes through three lifecycle phases:
//
//   - Register: called across ALL modules before any Boot. Only
//     container.Provide and config.SetDefault calls belong here.
//   - Boot: called after all modules register and framework services
//     initialize. Resolve services, wire routes, register event handlers.
//   - Shutdown: called in reverse registration order during graceful
//     shutdown. Best-effort — errors are collected, not fatal.
type Module interface {
	Name() string
	Register() error
	Boot() error
	Shutdown(ctx context.Context) error
}

// PostBooter is optionally implemented by modules that need to run setup
// after ALL modules have completed Boot(). Use it when setup depends on
// other modules being fully wired — data seeding, cache warm-up, or
// starting background workers that need multiple services.
//
// If PostBoot returns an error, all booted modules are shut down
// (same rollback semantics as Boot failure).
type PostBooter interface {
	PostBoot() error
}

// PreShutdowner is optionally implemented by modules that need to prepare
// for shutdown before individual module Shutdown() calls begin. All
// PreShutdown calls complete before any Shutdown call starts.
//
// Common use cases: stop accepting new work, drain in-flight requests,
// flush buffers, send shutdown notifications.
//
// PreShutdown is best-effort — errors are collected, not fatal.
type PreShutdowner interface {
	PreShutdown(ctx context.Context) error
}

// BaseModule provides no-op defaults for the Module interface. Embed it
// in concrete modules and override only the methods you need.
//
//	type OrderModule struct {
//	    app.BaseModule
//	}
//
//	func NewOrderModule() *OrderModule {
//	    return &OrderModule{BaseModule: app.BaseModule{ModuleName: "orders"}}
//	}
type BaseModule struct {
	ModuleName string
}

func (b *BaseModule) Name() string                     { return b.ModuleName }
func (b *BaseModule) Register() error                  { return nil }
func (b *BaseModule) Boot() error                      { return nil }
func (b *BaseModule) Shutdown(_ context.Context) error { return nil }

// ModuleGroup composes multiple modules into a single Module. Register and
// Boot delegate to children in order; Shutdown delegates in reverse order.
// Only children that completed Boot are shut down — if Boot fails partway
// through, already-booted children are rolled back immediately.
//
// This maps to Sunkern's hierarchical module structure (e.g., adminmod
// containing auth, usermgmt, logmgmt sub-modules).
type ModuleGroup struct {
	GroupName string
	Modules   []Module
	booted    []Module // children that completed Boot (for rollback)
}

func (g *ModuleGroup) Name() string { return g.GroupName }

func (g *ModuleGroup) Register() error {
	for _, m := range g.Modules {
		if err := m.Register(); err != nil {
			return fmt.Errorf("module %s/%s register: %w", g.GroupName, m.Name(), err)
		}
	}
	return nil
}

func (g *ModuleGroup) Boot() error {
	for _, m := range g.Modules {
		if err := m.Boot(); err != nil {
			// Rollback: shutdown children that already booted, in reverse.
			_ = g.shutdownBooted(context.Background())
			g.booted = nil // prevent double-shutdown if parent also calls Shutdown
			return fmt.Errorf("module %s/%s boot: %w", g.GroupName, m.Name(), err)
		}
		g.booted = append(g.booted, m)
	}
	return nil
}

func (g *ModuleGroup) Shutdown(ctx context.Context) error {
	return g.shutdownBooted(ctx)
}

// PostBoot delegates to children that implement PostBooter.
func (g *ModuleGroup) PostBoot() error {
	for _, m := range g.booted {
		pb, ok := m.(PostBooter)
		if !ok {
			continue
		}
		if err := pb.PostBoot(); err != nil {
			return fmt.Errorf("module %s/%s post-boot: %w", g.GroupName, m.Name(), err)
		}
	}
	return nil
}

// PreShutdown delegates to children that implement PreShutdowner.
func (g *ModuleGroup) PreShutdown(ctx context.Context) error {
	var errs []error
	for _, m := range g.booted {
		ps, ok := m.(PreShutdowner)
		if !ok {
			continue
		}
		if err := ps.PreShutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("module %s/%s pre-shutdown: %w",
				g.GroupName, m.Name(), err))
		}
	}
	return errors.Join(errs...)
}

func (g *ModuleGroup) shutdownBooted(ctx context.Context) error {
	var errs []error
	for i := len(g.booted) - 1; i >= 0; i-- {
		if err := g.booted[i].Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("module %s/%s shutdown: %w",
				g.GroupName, g.booted[i].Name(), err))
		}
	}
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// Optional module interfaces
// ---------------------------------------------------------------------------

// Tagger is optionally implemented by modules to provide categorization
// tags for admin introspection. Tags are free-form strings — common
// values include "builtin", "admin", "feature", "experimental".
type Tagger interface {
	Tags() []string
}

// DependencyDeclarer is optionally implemented by modules that depend on
// other modules being registered. The app validates all declared
// dependencies between the register and boot phases — if any named
// dependency is not registered (or is disabled), boot fails with a
// clear error. Additionally, each dependency must be registered before
// the dependent module (registration order = boot order).
//
// This is a validation-only mechanism — it does NOT reorder modules.
// Registration order still determines boot order.
type DependencyDeclarer interface {
	DependsOn() []string
}

// HealthCheckProvider is optionally implemented by modules that want to
// register health checks automatically during boot. The app calls
// HealthChecks() after a module successfully boots and registers all
// returned checkers.
//
// This avoids the need for modules to resolve *App from the container
// just to call AddHealthCheck.
//
//	func (m *CacheModule) HealthChecks() []app.HealthChecker {
//	    return []app.HealthChecker{
//	        app.CheckFunc{CheckerName: "cache", Fn: m.cache.Ping},
//	    }
//	}
type HealthCheckProvider interface {
	HealthChecks() []HealthChecker
}

// ---------------------------------------------------------------------------
// Conditional modules
// ---------------------------------------------------------------------------

// When conditionally includes a module. If condition is true, the module
// participates in all lifecycle phases normally. If false, the module is
// disabled — all lifecycle methods become no-ops, but it still appears
// in App.ModuleInfo() with status "disabled" and preserves the inner
// module's name, tags, and dependencies for introspection.
//
//	a.Use(app.When(os.Getenv("ENABLE_EMAIL") == "true", emailModule))
func When(condition bool, m Module) Module {
	if condition {
		return m
	}
	return &disabledModule{inner: m}
}

// disabledModule wraps a module excluded by When(false, ...). All
// lifecycle methods are no-ops. The inner module's metadata is preserved
// for introspection via App.ModuleInfo().
type disabledModule struct {
	inner Module
}

func (d *disabledModule) Name() string                     { return d.inner.Name() }
func (d *disabledModule) Register() error                  { return nil }
func (d *disabledModule) Boot() error                      { return nil }
func (d *disabledModule) Shutdown(_ context.Context) error { return nil }

// isDisabled reports whether m is a disabled wrapper from When(false, ...).
func isDisabled(m Module) bool {
	_, ok := m.(*disabledModule)
	return ok
}
