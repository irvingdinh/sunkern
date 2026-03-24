package container

import (
	"encoding/json"
	"sort"
)

// ServiceStatus represents the lifecycle state of a registered service.
type ServiceStatus int

const (
	// ServicePending means the provider has been registered but not yet called.
	ServicePending ServiceStatus = iota
	// ServiceBuilt means the provider succeeded and the instance is cached.
	ServiceBuilt
	// ServiceFailed means the provider ran and returned an error.
	ServiceFailed
)

// String returns a human-readable label for the status.
func (s ServiceStatus) String() string {
	switch s {
	case ServicePending:
		return "pending"
	case ServiceBuilt:
		return "built"
	case ServiceFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// MarshalJSON encodes the status as a JSON string ("pending", "built",
// "failed") instead of an integer.
func (s ServiceStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// ServiceInfo describes a single registered service for introspection.
// All fields have JSON tags for direct serialization in admin API responses.
type ServiceInfo struct {
	// Name is the Go type name used as the registry key (e.g., "*sqlite.DB").
	Name string `json:"name"`
	// Kind is "supplied" (pre-built value) or "provided" (lazy factory).
	Kind string `json:"kind"`
	// Status is the current lifecycle state.
	Status ServiceStatus `json:"status"`
	// Caller is the file:line where the service was registered.
	Caller string `json:"caller,omitempty"`
	// Error is the cached provider error, if any. Nil unless Status is
	// ServiceFailed. Excluded from JSON — use ErrorText for serialization.
	Error error `json:"-"`
	// ErrorText is the string representation of Error for JSON output.
	ErrorText string `json:"error,omitempty"`
}

// Len returns the number of registered services.
func (c *Container) Len() int {
	c.mu.RLock()
	n := len(c.services)
	c.mu.RUnlock()
	return n
}

// Keys returns the type names of all registered services, sorted
// alphabetically. This is useful for debugging and admin endpoints.
func (c *Container) Keys() []string {
	c.mu.RLock()
	keys := make([]string, 0, len(c.services))
	for k := range c.services {
		keys = append(keys, k)
	}
	c.mu.RUnlock()
	sort.Strings(keys)
	return keys
}

// Inspect returns information about all registered services, sorted by
// name. Each entry includes the type name, registration kind, lifecycle
// status, registration site, and any cached provider error. This is
// intended for admin dashboards and debugging — it does NOT trigger lazy
// initialization.
func (c *Container) Inspect() []ServiceInfo {
	c.mu.RLock()
	infos := make([]ServiceInfo, 0, len(c.services))
	for name, svc := range c.services {
		svc.mu.Lock()
		info := ServiceInfo{
			Name:   name,
			Kind:   svc.kind,
			Caller: svc.caller,
		}
		switch {
		case !svc.built:
			info.Status = ServicePending
		case svc.err != nil:
			info.Status = ServiceFailed
			info.Error = svc.err
			info.ErrorText = svc.err.Error()
		default:
			info.Status = ServiceBuilt
		}
		svc.mu.Unlock()
		infos = append(infos, info)
	}
	c.mu.RUnlock()

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return infos
}
