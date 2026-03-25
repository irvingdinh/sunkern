---
title: Avoid Overusing any
impact: MEDIUM
impactDescription: Using any removes compile-time type safety and pushes errors to runtime
tags: any, type-safety, generics
---

## Avoid Overusing any

**Impact: MEDIUM (Using any removes compile-time type safety and pushes errors to runtime)**

Only use `any` (alias for `interface{}`) when you genuinely need to accept any type, such as `json.Marshal`, `fmt.Println`, or similar marshaling/formatting functions. In all other cases, `any` removes compile-time safety and forces runtime type assertions or switches that are fragile and error-prone. Prefer generics or specific types.

**Incorrect (what's wrong):**

```go
func getKeys(m any) ([]any, error) {
    switch t := m.(type) {
    case map[string]int:
        keys := make([]any, 0, len(t))
        for k := range t {
            keys = append(keys, k)
        }
        return keys, nil
    case map[int]string:
        keys := make([]any, 0, len(t))
        for k := range t {
            keys = append(keys, k)
        }
        return keys, nil
    default:
        return nil, fmt.Errorf("unknown type: %T", t)
    }
}
```

**Correct (what's right):**

```go
func getKeys[K comparable, V any](m map[K]V) []K {
    keys := make([]K, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    return keys
}
```

Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #8.
