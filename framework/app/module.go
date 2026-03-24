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
