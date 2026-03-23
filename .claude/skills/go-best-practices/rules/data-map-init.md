---
title: Provide Size Hints When Initializing Maps
impact: MEDIUM
impactDescription: reduces allocations
tags: maps, initialization, performance
---

## Provide Size Hints When Initializing Maps

**Impact: MEDIUM (reduces allocations)**

A Go map is backed by hash table buckets. When a map grows beyond its current bucket capacity, the runtime must allocate new buckets and rehash all existing entries, which is expensive. If you know (or can estimate) the number of entries a map will hold, pass that count as the second argument to `make`. This pre-allocates enough buckets to hold the expected entries without rehashing, reducing both CPU and memory allocation overhead.

**Incorrect (what's wrong):**

```go
m := make(map[string]int) // No size hint
for _, v := range largeSlice {
	m[v.Key] = v.Value // Causes multiple rehashes as map grows
}
```

**Correct (what's right):**

```go
m := make(map[string]int, len(largeSlice))
for _, v := range largeSlice {
	m[v.Key] = v.Value
}
```
