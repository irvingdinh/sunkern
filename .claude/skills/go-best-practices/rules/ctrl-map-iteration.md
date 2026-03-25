---
title: Map Iteration Order Is Non-Deterministic
impact: MEDIUM
impactDescription: prevents order-dependent bugs
tags: maps, iteration, ordering
---

## Map Iteration Order Is Non-Deterministic

**Impact: MEDIUM (prevents order-dependent bugs)**

Map iteration in Go is intentionally non-deterministic. The language specification does not guarantee ordering by key, preservation of insertion order, or that an element added during iteration will be produced by the loop. Code that depends on map iteration order will produce inconsistent results across runs and is a source of flaky tests and subtle logic errors.

**Incorrect (what's wrong):**

```go
m := map[int]bool{0: true, 1: false, 2: true}
for k, v := range m {
	fmt.Printf("%d: %t\n", k, v) // Order varies between runs
}
```

**Correct (what's right):**

```go
m := map[int]bool{0: true, 1: false, 2: true}
keys := make([]int, 0, len(m))
for k := range m {
	keys = append(keys, k)
}
slices.Sort(keys)
for _, k := range keys {
	fmt.Printf("%d: %t\n", k, m[k]) // Deterministic order
}
```
