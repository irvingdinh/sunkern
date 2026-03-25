---
title: Use the Right Equality Check for Each Type
impact: MEDIUM
impactDescription: prevents incorrect comparisons
tags: comparison, reflect, equality
---

## Use the Right Equality Check for Each Type

**Impact: MEDIUM (prevents incorrect comparisons)**

Go's `==` operator works for comparable types: booleans, numerics, strings, pointers, channels, and structs where all fields are themselves comparable. However, slices, maps, and structs containing non-comparable fields cannot use `==` and will cause a compile error. For these types, use `reflect.DeepEqual`, which performs a recursive comparison but carries a performance cost due to reflection. Starting with Go 1.21, prefer the type-safe `slices.Equal` and `maps.Equal` functions from the standard library, which avoid reflection overhead.

**Incorrect (what's wrong):**

```go
s1 := []int{1, 2, 3}
s2 := []int{1, 2, 3}
fmt.Println(s1 == s2) // Compile error: slice can only be compared to nil
```

**Correct (what's right):**

```go
s1 := []int{1, 2, 3}
s2 := []int{1, 2, 3}
fmt.Println(reflect.DeepEqual(s1, s2)) // true

// Or use slices.Equal from Go 1.21+:
fmt.Println(slices.Equal(s1, s2)) // true
```
