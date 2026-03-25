package container

import (
	"fmt"
	"runtime"
)

// ProvideTo registers a lazy singleton provider for type T on a specific
// container. This is the non-global equivalent of Provide — use it with
// isolated containers in tests or when multiple containers are needed.
func ProvideTo[T any](c *Container, provider func() (T, error)) {
	provideToContainer[T](c, provider, captureCallerAt(1))
}

// SupplyTo registers a pre-built value for type T on a specific container.
// This is the non-global equivalent of Supply.
func SupplyTo[T any](c *Container, value T) {
	supplyToContainer[T](c, value, captureCallerAt(1))
}

// OverrideIn replaces the registration for type T on a specific container
// with a new lazy provider. This is the non-global equivalent of Override.
func OverrideIn[T any](c *Container, provider func() (T, error)) {
	overrideToContainer[T](c, provider, captureCallerAt(1))
}

// OverrideSupplyIn replaces the registration for type T on a specific
// container with a pre-built value. This is the non-global equivalent of
// OverrideSupply.
func OverrideSupplyIn[T any](c *Container, value T) {
	overrideSupplyToContainer[T](c, value, captureCallerAt(1))
}

// captureCallerAt returns the file:line of the caller at the given skip
// depth above the caller of captureCallerAt itself. skip=1 means the
// caller's caller (the typical exported→internal chain).
func captureCallerAt(skip int) string {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown"
	}
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' {
			file = file[i+1:]
			break
		}
	}
	return fmt.Sprintf("%s:%d", file, line)
}

// provideToContainer registers a lazy singleton provider for type T.
// The provider is not called until the first Make or MustMake for T.
// Panics if a service for T is already registered. The caller string
// identifies the registration site for debugging.
func provideToContainer[T any](c *Container, provider func() (T, error), caller string) {
	name := typeName[T]()

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, exists := c.services[name]; exists {
		panic(fmt.Sprintf("container: duplicate provider for %s (registered at %s, duplicate at %s)", name, existing.caller, caller))
	}

	c.services[name] = &service{
		provider: func() (any, error) {
			return provider()
		},
		kind:   kindProvided,
		caller: caller,
	}
}

// supplyToContainer registers a pre-built value for type T. The value is
// available immediately — no provider is called on resolution. Panics if
// a service for T is already registered.
func supplyToContainer[T any](c *Container, value T, caller string) {
	name := typeName[T]()

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, exists := c.services[name]; exists {
		panic(fmt.Sprintf("container: duplicate provider for %s (registered at %s, duplicate at %s)", name, existing.caller, caller))
	}

	c.services[name] = &service{
		instance: value,
		built:    true,
		kind:     kindSupplied,
		caller:   caller,
	}
}

// overrideToContainer replaces the registration for type T with a new lazy
// provider. If T was already resolved, the cached instance is discarded and
// the new provider runs on the next resolution. If T is not yet registered,
// it is registered as new. This is intended for testing and environment
// switching — normal application code should use provideToContainer.
func overrideToContainer[T any](c *Container, provider func() (T, error), caller string) {
	name := typeName[T]()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.services[name] = &service{
		provider: func() (any, error) {
			return provider()
		},
		kind:   kindProvided,
		caller: caller,
	}
}

// overrideSupplyToContainer replaces the registration for type T with a
// pre-built value. Same semantics as overrideToContainer but without a
// lazy provider.
func overrideSupplyToContainer[T any](c *Container, value T, caller string) {
	name := typeName[T]()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.services[name] = &service{
		instance: value,
		built:    true,
		kind:     kindSupplied,
		caller:   caller,
	}
}
