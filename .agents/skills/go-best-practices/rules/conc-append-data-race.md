---
title: Append Data Race with Shared Slices
impact: CRITICAL
impactDescription: causes data races
tags: append, slice, data-race, concurrency
---

## Append Data Race with Shared Slices

**Impact: CRITICAL (causes data races)**

Appending to a shared slice with remaining capacity is a data race — both goroutines write to the same backing array index.

**Incorrect (what's wrong):**

```go
s := make([]int, 0, 1) // Length 0, capacity 1
go func() { s1 := append(s, 1); fmt.Println(s1) }()
go func() { s2 := append(s, 1); fmt.Println(s2) }()
// Data race: both write to index 0 of same backing array
```

**Correct (what's right):**

```go
s := make([]int, 1) // Length == capacity, append allocates new arrays
go func() { s1 := append(s, 1); fmt.Println(s1) }()
go func() { s2 := append(s, 1); fmt.Println(s2) }()
// Or: create a copy for each goroutine
```
