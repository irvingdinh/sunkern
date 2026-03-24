package container

// global is the package-level singleton container. All exported generic
// functions operate on this instance. Call Reset to clear state between
// tests — the same pattern as config.Load.
var global = New()

// Provide registers a lazy singleton provider for type T. The provider is
// called at most once, on the first Make or MustMake for T. Panics if T
// is already registered.
func Provide[T any](provider func() (T, error)) {
	provideToContainer[T](global, provider)
}

// Supply registers a pre-built value for type T. The value is available
// immediately — no provider is called on resolution. Panics if T is
// already registered.
func Supply[T any](value T) {
	supplyToContainer[T](global, value)
}

// Make resolves type T from the global container. On first call the
// provider runs and the result is cached. Returns an error if T is not
// registered or if the provider fails.
func Make[T any]() (T, error) {
	return makeFromContainer[T](global)
}

// MustMake resolves type T or panics. Use inside providers where a
// missing dependency is a programming error, not a runtime condition.
func MustMake[T any]() T {
	return mustMakeFromContainer[T](global)
}

// Override replaces the registration for type T with a new lazy provider.
// Unlike Provide, this does not panic on duplicate registration — it
// silently replaces the existing provider (and discards any cached instance).
// If T is not registered, it is registered as new. Intended for testing and
// environment switching.
func Override[T any](provider func() (T, error)) {
	overrideToContainer[T](global, provider)
}

// OverrideSupply replaces the registration for type T with a pre-built
// value. Same semantics as Override but without a lazy provider.
func OverrideSupply[T any](value T) {
	overrideSupplyToContainer[T](global, value)
}

// Has reports whether type T is registered.
func Has[T any]() bool {
	return hasInContainer[T](global)
}

// Len returns the number of registered services in the global container.
func Len() int {
	return global.Len()
}

// Keys returns the type names of all registered services in the global
// container, sorted alphabetically.
func Keys() []string {
	return global.Keys()
}

// Inspect returns information about all registered services in the global
// container, sorted by name. Does not trigger lazy initialization.
func Inspect() []ServiceInfo {
	return global.Inspect()
}

// AppendHook adds a lifecycle hook to the global container.
func AppendHook(h Hook) {
	global.AppendHook(h)
}

// Reset replaces the global container with a fresh instance. Intended for
// tests and the application boot sequence.
func Reset() {
	global = New()
}

// Global returns the package-level container instance. Exported for the
// rare case where framework code needs to pass the container explicitly
// (e.g., to App). Normal application code should use the package-level
// functions instead.
func Global() *Container {
	return global
}
