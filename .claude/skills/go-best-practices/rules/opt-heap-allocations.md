---
title: Reduce Heap Allocations
impact: MEDIUM
impactDescription: reduces GC pressure
tags: heap, stack, allocations, escape-analysis, sync-pool
---

## Reduce Heap Allocations

**Impact: MEDIUM (reduces GC pressure)**

Stack allocations are essentially free -- they are reclaimed when the function returns. Heap allocations require garbage collection, which consumes CPU and can cause latency spikes. Understanding escape analysis helps you keep allocations on the stack.

A variable escapes to the heap when: (A) its pointer is returned or stored beyond the function's lifetime, (B) it is too large for the stack, or (C) the compiler cannot prove it stays local (e.g., sent to an interface). Use `go build -gcflags="-m"` to see what escapes.

For high-frequency allocations, `sync.Pool` recycles objects to avoid repeated allocation and GC.

**Incorrect (what's wrong):**

```go
// Returning a pointer causes heap escape
func createUser(name string) *User {
	u := User{Name: name} // Escapes to heap because pointer is returned
	return &u
}

// Allocating a buffer on every call
func process(data []byte) ([]byte, error) {
	buf := make([]byte, 1024) // Heap allocation every call
	n := transform(data, buf)
	return buf[:n], nil
}

// Sharing up the call stack via pointer
func readConfig() (*Config, error) {
	var cfg Config          // Escapes to heap
	err := json.Unmarshal(data, &cfg)
	return &cfg, err
}
```

**Correct (what's right):**

```go
// Caller provides memory — stays on stack
func createUser(name string, u *User) {
	u.Name = name
}

// Usage:
var u User
createUser("Alice", &u) // u stays on the caller's stack

// sync.Pool for high-frequency allocations
var bufPool = sync.Pool{
	New: func() any { return make([]byte, 1024) },
}

func process(data []byte) ([]byte, error) {
	buf := bufPool.Get().([]byte)
	defer bufPool.Put(buf)
	buf = buf[:1024] // Reset length
	n := transform(data, buf)
	result := make([]byte, n)
	copy(result, buf[:n])
	return result, nil
}

// Check escape analysis:
// go build -gcflags="-m" ./...
// ./main.go:10:6: moved to heap: cfg
// ./main.go:15:2: &u escapes to heap

// Common escapes to watch for:
// - Returning pointers from functions
// - Storing pointers in long-lived structs
// - Passing values to interface{}/any parameters
// - Closures capturing local variables
```
