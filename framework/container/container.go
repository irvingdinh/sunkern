package container

import (
	"context"
	"reflect"
	"sync"
)

// resolveState tracks the resolution chain for a single goroutine during
// service initialization. Used for circular dependency detection and
// dependency graph building.
type resolveState struct {
	stack []string                       // service names being built, in order
	deps  map[string]map[string]struct{} // parent name → set of direct dep names
}

// Registration kind constants.
const (
	kindProvided = "provided" // lazy singleton with factory function
	kindSupplied = "supplied" // pre-built value, available immediately
)

// service holds a lazily-initialized singleton. The provider is called at
// most once; subsequent resolutions return the cached instance. If the
// provider fails, the error is cached and returned on all future calls —
// the provider is never retried.
type service struct {
	provider func() (any, error) // nil for Supply'd values
	instance any
	err      error    // cached provider error (nil on success)
	built    bool
	kind     string   // kindProvided or kindSupplied
	caller   string   // "file.go:42" — registration call site
	deps     []string // direct dependencies resolved during build (sorted, unique)
	mu       sync.Mutex // protects lazy init (per-service, not global)
}

// Hook pairs an optional start callback with an optional stop callback.
// Hooks are executed in registration order on start and reverse order on stop.
// Name is optional — when set, it appears in error messages instead of the
// hook's positional index (e.g., "starting hook 'http-server'" vs "starting
// hook 2").
type Hook struct {
	Name    string
	OnStart func(ctx context.Context) error
	OnStop  func(ctx context.Context) error
}

// Container is a type-keyed singleton registry. Services are registered via
// the package-level Provide and Supply functions, and resolved via Make and
// MustMake. The Container itself is exported for isolated testing; normal
// application code uses the package-level facade that operates on a global
// instance.
type Container struct {
	mu       sync.RWMutex
	services map[string]*service
	hooks    []Hook

	// Resolution tracking: per-goroutine resolution chain for circular
	// dependency detection and dependency graph building. Only active
	// during provider execution (boot phase); empty at runtime.
	resolveMu sync.Mutex
	resolving map[int64]*resolveState
}

// New returns an empty container.
func New() *Container {
	return &Container{
		services:  make(map[string]*service),
		resolving: make(map[int64]*resolveState),
	}
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

// typeName returns the unique string key for a Go type. Two registrations
// of the same concrete type always produce the same key.
func typeName[T any]() string {
	return reflect.TypeFor[T]().String()
}
