package container

import "fmt"

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
