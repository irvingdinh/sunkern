package container

import (
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
)

// MakeFrom resolves type T from a specific container. This is the non-global
// equivalent of Make — use it with isolated containers in tests.
func MakeFrom[T any](c *Container) (T, error) {
	return makeFromContainer[T](c)
}

// MustMakeFrom resolves type T from a specific container or panics. This is
// the non-global equivalent of MustMake.
func MustMakeFrom[T any](c *Container) T {
	return mustMakeFromContainer[T](c)
}

// HasIn reports whether type T is registered in a specific container. This
// is the non-global equivalent of Has.
func HasIn[T any](c *Container) bool {
	return hasInContainer[T](c)
}

// makeFromContainer resolves type T from the container. On first call the
// provider runs and the result is cached; subsequent calls return the
// cached singleton. Returns an error if T is not registered or if the
// provider fails.
//
// Circular dependencies are detected before acquiring the per-service lock,
// preventing deadlocks. When a cycle is found (e.g., A's provider resolves B,
// whose provider resolves A), makeFromContainer panics with a clear message
// showing the full cycle chain.
//
// Dependencies are tracked during provider execution: when provider A resolves
// service B, B is recorded as a direct dependency of A. These deps are stored
// on the service and exposed via Inspect() and DependencyGraph().
func makeFromContainer[T any](c *Container) (T, error) {
	name := typeName[T]()
	var zero T

	c.mu.RLock()
	svc, exists := c.services[name]
	c.mu.RUnlock()

	if !exists {
		return zero, fmt.Errorf("container: service not found: %s", name)
	}

	// Check for circular dependency BEFORE acquiring the service lock.
	// This prevents deadlocks when provider A → B → A would re-lock A's mutex.
	if cycle := c.checkCircularIfResolving(name); cycle != "" {
		panic(fmt.Sprintf("container: circular dependency: %s", cycle))
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	// Fast path: service already built (cached singleton).
	if svc.built {
		if svc.err != nil {
			return zero, fmt.Errorf("container: creating %s: %w", name, svc.err)
		}
		// Record as dependency of whatever is currently being built.
		c.recordDepIfResolving(name)
		return svc.instance.(T), nil
	}

	// Slow path: first resolution — run provider.
	if err := c.runProvider(svc, name); err != nil {
		return zero, fmt.Errorf("container: creating %s: %w", name, err)
	}

	return svc.instance.(T), nil
}

// runProvider executes the provider for a service that has not yet been built.
// The caller MUST hold svc.mu and MUST have verified svc.built == false.
// On success, svc.instance and svc.built are set. On failure, svc.err and
// svc.built are set (the provider is never retried).
//
// Dependencies are tracked: if this provider resolves other services via
// Make/MustMake, those are recorded as direct dependencies of name.
func (c *Container) runProvider(svc *service, name string) error {
	gid := goID()
	c.recordDep(gid, name)
	c.pushResolve(gid, name)

	// Ensure resolve stack is cleaned up even if the provider panics.
	popped := false
	defer func() {
		if !popped {
			c.popResolve(gid)
		}
	}()

	instance, err := svc.provider()
	popped = true
	svc.deps = c.popResolve(gid)

	if err != nil {
		// Cache the error so the provider is never retried. The service is
		// considered permanently failed — the app should restart to retry.
		svc.err = err
		svc.built = true
		return err
	}

	svc.instance = instance
	svc.built = true
	return nil
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

// buildByName resolves a service by its string name. Used by BuildAll where
// the concrete Go type is not known at compile time. Applies circular
// detection and dependency tracking identically to makeFromContainer.
func (c *Container) buildByName(name string) error {
	c.mu.RLock()
	svc, exists := c.services[name]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("container: service not found: %s", name)
	}

	if cycle := c.checkCircularIfResolving(name); cycle != "" {
		panic(fmt.Sprintf("container: circular dependency: %s", cycle))
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	if svc.built {
		if svc.err != nil {
			return fmt.Errorf("container: creating %s: %w", name, svc.err)
		}
		c.recordDepIfResolving(name)
		return nil
	}

	return c.runProvider(svc, name)
}

// BuildAll eagerly resolves all pending (unbuilt) providers in the container.
// Services are built in alphabetical order by type name for deterministic
// behavior. If a provider resolves other services internally (via Make or
// MustMake), those are built on demand as usual — BuildAll simply ensures
// nothing is left pending.
//
// Returns an error (via errors.Join) if any providers fail. Failed services
// have their errors cached as usual — they are not retried on subsequent
// Make calls. Services that were already built (including supplied values)
// are skipped.
//
// Use BuildAll at the end of the boot phase to fail fast on provider errors
// instead of discovering them lazily at first request time.
func (c *Container) BuildAll() error {
	c.mu.RLock()
	var pending []string
	for name, svc := range c.services {
		svc.mu.Lock()
		if !svc.built {
			pending = append(pending, name)
		}
		svc.mu.Unlock()
	}
	c.mu.RUnlock()

	sort.Strings(pending)

	var errs []error
	for _, name := range pending {
		if err := c.buildByName(name); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// Resolution tracking — circular detection & dependency graph
// ---------------------------------------------------------------------------

// goID returns the current goroutine's ID by parsing runtime.Stack output.
// Only called during service initialization (not on the hot path for cached
// resolutions), so the small allocation and parse overhead is negligible.
func goID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// Output format: "goroutine 18 [running]:\n..."
	// Skip "goroutine " prefix, parse digits.
	s := buf[len("goroutine "):n]
	var id int64
	for _, b := range s {
		if b < '0' || b > '9' {
			break
		}
		id = id*10 + int64(b-'0')
	}
	return id
}

// checkCircularIfResolving checks if resolving name would create a circular
// dependency on the current goroutine's resolution stack. Returns the cycle
// chain (e.g., "A → B → A") or "" if no cycle. Called before acquiring the
// per-service lock to prevent deadlocks.
//
// Fast path: if no goroutine is currently resolving, returns "" without
// calling goID() — zero overhead after boot.
func (c *Container) checkCircularIfResolving(name string) string {
	c.resolveMu.Lock()
	if len(c.resolving) == 0 {
		c.resolveMu.Unlock()
		return ""
	}
	c.resolveMu.Unlock()

	gid := goID()
	return c.checkCircular(gid, name)
}

// checkCircular inspects the given goroutine's resolution stack for name.
// If found, returns the cycle chain; otherwise returns "".
func (c *Container) checkCircular(gid int64, name string) string {
	c.resolveMu.Lock()
	defer c.resolveMu.Unlock()

	rs := c.resolving[gid]
	if rs == nil {
		return ""
	}

	for i, s := range rs.stack {
		if s == name {
			// Build cycle: stack[i:] → name
			cycle := make([]string, len(rs.stack)-i+1)
			copy(cycle, rs.stack[i:])
			cycle[len(cycle)-1] = name
			return strings.Join(cycle, " → ")
		}
	}
	return ""
}

// pushResolve pushes name onto the current goroutine's resolution stack.
func (c *Container) pushResolve(gid int64, name string) {
	c.resolveMu.Lock()
	defer c.resolveMu.Unlock()

	rs := c.resolving[gid]
	if rs == nil {
		rs = &resolveState{deps: make(map[string]map[string]struct{})}
		c.resolving[gid] = rs
	}
	rs.stack = append(rs.stack, name)
}

// popResolve pops the top name from the current goroutine's resolution
// stack and returns the collected deps for that name as a sorted slice.
// Cleans up the goroutine entry when the stack becomes empty.
func (c *Container) popResolve(gid int64) []string {
	c.resolveMu.Lock()
	defer c.resolveMu.Unlock()

	rs := c.resolving[gid]
	if rs == nil || len(rs.stack) == 0 {
		return nil
	}

	// Pop
	name := rs.stack[len(rs.stack)-1]
	rs.stack = rs.stack[:len(rs.stack)-1]

	// Collect deps
	depSet := rs.deps[name]
	delete(rs.deps, name)

	// Clean up goroutine entry when stack is empty
	if len(rs.stack) == 0 {
		delete(c.resolving, gid)
	}

	if len(depSet) == 0 {
		return nil
	}

	// Convert set to sorted slice for deterministic output
	deps := make([]string, 0, len(depSet))
	for d := range depSet {
		deps = append(deps, d)
	}
	sort.Strings(deps)
	return deps
}

// recordDep records name as a direct dependency of the service currently
// being built on the given goroutine. No-op if the goroutine isn't building
// anything or if name is the service itself.
func (c *Container) recordDep(gid int64, name string) {
	c.resolveMu.Lock()
	defer c.resolveMu.Unlock()

	rs := c.resolving[gid]
	if rs == nil || len(rs.stack) == 0 {
		return
	}

	parent := rs.stack[len(rs.stack)-1]
	if parent == name {
		return // skip self-reference
	}

	if rs.deps[parent] == nil {
		rs.deps[parent] = make(map[string]struct{})
	}
	rs.deps[parent][name] = struct{}{}
}

// recordDepIfResolving records name as a dependency of whatever is currently
// being built on this goroutine. Used for cached-hit paths where the service
// is already built but we still need to track the dependency edge.
//
// Fast path: if no goroutine is currently resolving, returns without calling
// goID() — zero overhead after boot.
func (c *Container) recordDepIfResolving(name string) {
	c.resolveMu.Lock()
	if len(c.resolving) == 0 {
		c.resolveMu.Unlock()
		return
	}
	c.resolveMu.Unlock()

	gid := goID()
	c.recordDep(gid, name)
}
