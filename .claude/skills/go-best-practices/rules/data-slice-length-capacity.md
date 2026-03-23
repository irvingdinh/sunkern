---
title: Understand Slice Length vs Capacity
impact: HIGH
impactDescription: foundational slice knowledge
tags: slices, length, capacity, backing-array
---

## Understand Slice Length vs Capacity

**Impact: HIGH (foundational slice knowledge)**

A Go slice is a three-field structure: a pointer to a backing array, a length (the number of accessible elements), and a capacity (the total size of the backing array from the slice's start). You can only index up to length minus one; accessing beyond that panics even if capacity is larger. Use `append` to grow a slice within its existing capacity. When `append` exceeds the capacity, Go allocates a new, larger backing array and copies elements over.

Slicing with `s[low:high]` creates a new slice header that shares the same backing array. Modifications through one slice are visible through the other until a reallocation occurs.

**Incorrect (what's wrong):**

```go
s := make([]int, 3, 6)
s[4] = 0 // panic: index out of range [4] with length 3
```

**Correct (what's right):**

```go
s := make([]int, 3, 6)
s = append(s, 2) // Uses existing capacity, length becomes 4

// Slicing shares the backing array:
s1 := []int{1, 2, 3, 4, 5}
s2 := s1[1:3] // s2 is [2, 3], shares backing array with s1

// Appending beyond capacity allocates a new backing array:
s3 := make([]int, 0, 2)
s3 = append(s3, 1, 2)    // Uses existing capacity
s3 = append(s3, 3)        // Exceeds capacity — new backing array allocated
```
