---
title: Check Slice Emptiness with len, Not nil
impact: MEDIUM
impactDescription: handles nil and empty correctly
tags: slices, nil, empty, length
---

## Check Slice Emptiness with len, Not nil

**Impact: MEDIUM (handles nil and empty correctly)**

A slice can be empty in two ways: it can be nil (`var s []int`) or it can be a non-nil slice with zero length (`s := []int{}`). Checking `s == nil` only catches the first case and silently passes through empty non-nil slices. Always use `len(s) == 0` to check for emptiness, as it correctly handles both nil and empty slices. The same principle applies to maps: use `len(m) == 0` instead of `m == nil`.

**Incorrect (what's wrong):**

```go
func handleResults(results []int) {
	if results == nil {
		return // Misses empty non-nil slices
	}
}
```

**Correct (what's right):**

```go
func handleResults(results []int) {
	if len(results) == 0 {
		return // Handles both nil and empty
	}
}
```
