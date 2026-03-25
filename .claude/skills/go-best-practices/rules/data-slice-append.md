---
title: Prevent Side Effects When Appending to Shared Slices
impact: HIGH
impactDescription: prevents mutation of shared data
tags: slices, append, side-effects
---

## Prevent Side Effects When Appending to Shared Slices

**Impact: HIGH (prevents mutation of shared data)**

When you create a sub-slice (e.g., `s2 := s1[:2]`), the new slice shares the same backing array as the original. If the sub-slice has remaining capacity (capacity > length), calling `append` on it will overwrite elements in the original slice rather than allocating a new array. This is a subtle and dangerous bug because the mutation happens silently.

Use a full slice expression (`s[low:high:max]`) to set the capacity equal to the length, forcing `append` to allocate a new backing array. Alternatively, use `copy` to create a fully independent slice.

**Incorrect (what's wrong):**

```go
s1 := []int{1, 2, 3}
s2 := s1[:2]
s2 = append(s2, 10) // Overwrites s1[2] — now s1 is [1, 2, 10]
```

**Correct (what's right):**

```go
// Option 1: Full slice expression — capacity == length
s1 := []int{1, 2, 3}
s2 := s1[:2:2] // capacity == length, append will allocate new array
s2 = append(s2, 10) // s1 unchanged: [1, 2, 3]

// Option 2: Copy to create a fully independent slice
s1 := []int{1, 2, 3}
s2 := make([]int, 2)
copy(s2, s1[:2])
s2 = append(s2, 10) // s1 unchanged: [1, 2, 3]
```
