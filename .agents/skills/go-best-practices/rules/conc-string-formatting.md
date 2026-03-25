---
title: String Formatting Deadlocks
impact: HIGH
impactDescription: prevents deadlocks
tags: deadlock, formatting, mutex, stringer
---

## String Formatting Deadlocks

**Impact: HIGH (prevents deadlocks)**

String formatting (%s, %v) can call String() methods, which may try to acquire locks already held — causing deadlocks.

**Incorrect (what's wrong):**

```go
type Customer struct {
    mu  sync.RWMutex
    id  string
    age int
}
func (c *Customer) UpdateAge(age int) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    if age < 0 {
        return fmt.Errorf("age should be positive for customer %v", c) // Calls String() → deadlock
    }
    c.age = age; return nil
}
func (c *Customer) String() string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return fmt.Sprintf("id %s, age %d", c.id, c.age)
}
```

**Correct (what's right):**

```go
func (c *Customer) UpdateAge(age int) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    if age < 0 {
        return fmt.Errorf("age should be positive for customer id %s", c.id) // Access field directly
    }
    c.age = age; return nil
}
```
