---
title: Mutex Protection for Slices and Maps
impact: CRITICAL
impactDescription: prevents false safety
tags: mutex, slices, maps, data-race, copy
---

## Mutex Protection for Slices and Maps

**Impact: CRITICAL (prevents false safety)**

Assigning a map or slice to a new variable copies the header, not the data. Both variables reference the same underlying data. Mutex-protecting only the assignment doesn't prevent races on the data.

**Incorrect (what's wrong):**

```go
func (c *Cache) AverageBalance() float64 {
    c.mu.RLock()
    balances := c.balances // Copies map header, NOT data
    c.mu.RUnlock()
    sum := 0.0
    for _, b := range balances { sum += b } // Data race with concurrent writes
    return sum / float64(len(balances))
}
```

**Correct (what's right):**

```go
func (c *Cache) AverageBalance() float64 {
    c.mu.RLock()
    defer c.mu.RUnlock()
    sum := 0.0
    for _, b := range c.balances { sum += b }
    return sum / float64(len(c.balances))
}
// Or: deep copy with maps.Clone() inside the lock, then iterate outside
```
