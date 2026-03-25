---
title: Goroutine Lifecycle Management
impact: CRITICAL
impactDescription: prevents goroutine leaks
tags: goroutines, leaks, lifecycle, cleanup
---

## Goroutine Lifecycle Management

**Impact: CRITICAL (prevents goroutine leaks)**

Every goroutine must have a plan to stop. Use context cancellation and wait for cleanup before exiting.

**Incorrect (what's wrong):**

```go
func newWatcher() {
    w := watcher{}
    go w.watch() // No way to stop, no cleanup guarantee
}
```

**Correct (what's right):**

```go
func main() {
    w := newWatcher()
    defer w.close() // Blocks until resources are freed
    // Run application
}
func newWatcher() *watcher {
    w := &watcher{}
    go w.watch()
    return w
}
func (w *watcher) close() { /* signal stop and wait for cleanup */ }
```
