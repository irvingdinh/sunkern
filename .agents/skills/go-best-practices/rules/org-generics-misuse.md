---
title: Don't Misuse Generics
impact: MEDIUM
impactDescription: Unnecessary generics add complexity without reducing boilerplate
tags: generics, type-parameters, abstraction
---

## Don't Misuse Generics

**Impact: MEDIUM (Unnecessary generics add complexity without reducing boilerplate)**

Don't use generics prematurely. Use them when you see concrete boilerplate to eliminate: data structures (trees, linked lists), slice/map/channel operations (filter, map, merge), and factoring out type-specific behavior (sort). Don't use generics when the type parameter is only used to call a method on it — in that case, an interface parameter is simpler and more readable.

**Incorrect (what's wrong):**

```go
// Unnecessary generic — the type parameter is only used to call Write
func foo[T io.Writer](w T) {
    b := getBytes()
    _, _ = w.Write(b)
}
```

**Correct (what's right):**

```go
// Interface parameter is simpler and more idiomatic
func foo(w io.Writer) {
    b := getBytes()
    _, _ = w.Write(b)
}
```

When generics are appropriate:

```go
// Good: eliminates boilerplate for any map type
func Keys[K comparable, V any](m map[K]V) []K {
    keys := make([]K, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    return keys
}

// Good: generic data structure
type Stack[T any] struct {
    items []T
}

func (s *Stack[T]) Push(item T) {
    s.items = append(s.items, item)
}

func (s *Stack[T]) Pop() (T, bool) {
    if len(s.items) == 0 {
        var zero T
        return zero, false
    }
    item := s.items[len(s.items)-1]
    s.items = s.items[:len(s.items)-1]
    return item, true
}
```

Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #9.
