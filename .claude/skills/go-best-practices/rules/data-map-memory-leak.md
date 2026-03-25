---
title: Prevent Memory Leaks from Map Bucket Growth
impact: HIGH
impactDescription: prevents memory leaks
tags: maps, memory-leak, buckets, gc
---

## Prevent Memory Leaks from Map Bucket Growth

**Impact: HIGH (prevents memory leaks)**

A Go map grows its internal bucket array as entries are added, but it never shrinks the bucket array when entries are deleted. Even after deleting every key, the map retains its peak number of buckets. This means a map that temporarily held a million entries will continue to consume the memory for those buckets indefinitely, even when empty.

To reclaim memory, periodically re-create the map by copying live entries into a fresh map. Alternatively, use pointer values (e.g., `map[int]*[128]byte` instead of `map[int][128]byte`) to reduce the per-bucket memory cost, since pointers are much smaller than large value types.

**Incorrect (what's wrong):**

```go
m := make(map[int][128]byte)
for i := 0; i < 1_000_000; i++ {
	m[i] = [128]byte{}
}
for i := 0; i < 1_000_000; i++ {
	delete(m, i)
}
runtime.GC() // Still ~293 MB — buckets not freed
```

**Correct (what's right):**

```go
// Option 1: Re-create the map periodically
newMap := make(map[int][128]byte, len(m))
for k, v := range m {
	newMap[k] = v
}
m = newMap // Old map can be GC'd

// Option 2: Use pointer values to reduce bucket memory
m := make(map[int]*[128]byte)
```
