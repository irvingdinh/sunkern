package container

import "fmt"

// makeFromContainer resolves type T from the container. On first call the
// provider runs and the result is cached; subsequent calls return the
// cached singleton. Returns an error if T is not registered or if the
// provider fails.
func makeFromContainer[T any](c *Container) (T, error) {
	name := typeName[T]()
	var zero T

	c.mu.RLock()
	svc, exists := c.services[name]
	c.mu.RUnlock()

	if !exists {
		return zero, fmt.Errorf("container: service not found: %s", name)
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	if svc.built {
		if svc.err != nil {
			return zero, fmt.Errorf("container: creating %s: %w", name, svc.err)
		}
		return svc.instance.(T), nil
	}

	instance, err := svc.provider()
	if err != nil {
		// Cache the error so the provider is never retried. The service is
		// considered permanently failed — the app should restart to retry.
		svc.err = err
		svc.built = true
		return zero, fmt.Errorf("container: creating %s: %w", name, err)
	}

	svc.instance = instance
	svc.built = true

	return instance.(T), nil
}

// mustMakeFromContainer resolves type T or panics. Use inside providers
// where a missing dependency is a programming error.
func mustMakeFromContainer[T any](c *Container) T {
	v, err := makeFromContainer[T](c)
	if err != nil {
		panic(err)
	}
	return v
}

// hasInContainer reports whether type T is registered (built or not).
func hasInContainer[T any](c *Container) bool {
	name := typeName[T]()
	c.mu.RLock()
	_, exists := c.services[name]
	c.mu.RUnlock()
	return exists
}
