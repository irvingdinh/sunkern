package container

import "fmt"

// provideToContainer registers a lazy singleton provider for type T.
// The provider is not called until the first Make or MustMake for T.
// Panics if a service for T is already registered.
func provideToContainer[T any](c *Container, provider func() (T, error)) {
	name := typeName[T]()

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.services[name]; exists {
		panic(fmt.Sprintf("container: duplicate provider for %s", name))
	}

	c.services[name] = &service{
		provider: func() (any, error) {
			return provider()
		},
	}
}

// supplyToContainer registers a pre-built value for type T. The value is
// available immediately — no provider is called on resolution. Panics if
// a service for T is already registered.
func supplyToContainer[T any](c *Container, value T) {
	name := typeName[T]()

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.services[name]; exists {
		panic(fmt.Sprintf("container: duplicate provider for %s", name))
	}

	c.services[name] = &service{
		instance: value,
		built:    true,
	}
}
