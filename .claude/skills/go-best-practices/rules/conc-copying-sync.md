---
title: Never Copy sync Types
impact: HIGH
impactDescription: causes undefined behavior
tags: sync, copy, mutex, waitgroup
---

## Never Copy sync Types

**Impact: HIGH (causes undefined behavior)**

sync types (Mutex, WaitGroup, Cond, etc.) must NEVER be copied after first use. Pass by pointer. Use go vet to detect this.

**Incorrect (what's wrong):**

```go
type Service struct { mu sync.Mutex }
func (s Service) DoWork() { // Value receiver copies the mutex
    s.mu.Lock()
    defer s.mu.Unlock()
    // ...
}
```

**Correct (what's right):**

```go
type Service struct { mu sync.Mutex }
func (s *Service) DoWork() { // Pointer receiver — no copy
    s.mu.Lock()
    defer s.mu.Unlock()
    // ...
}
```
