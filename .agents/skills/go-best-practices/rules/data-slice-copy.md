---
title: Ensure Destination Slice Has Sufficient Length for copy
impact: MEDIUM
impactDescription: prevents partial copies
tags: slices, copy, length
---

## Ensure Destination Slice Has Sufficient Length for copy

**Impact: MEDIUM (prevents partial copies)**

The built-in `copy` function copies `min(len(dst), len(src))` elements from the source to the destination slice. If the destination slice has zero length (including nil slices), `copy` copies nothing without any error or warning. This is a common mistake: declaring a nil destination and expecting `copy` to populate it. Always allocate the destination with the correct length before calling `copy`.

**Incorrect (what's wrong):**

```go
src := []int{0, 1, 2}
var dst []int
copy(dst, src) // Copies 0 elements — dst is empty
```

**Correct (what's right):**

```go
src := []int{0, 1, 2}
dst := make([]int, len(src))
copy(dst, src) // Copies all 3 elements
```
