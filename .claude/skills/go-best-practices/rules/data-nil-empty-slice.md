---
title: Distinguish Nil Slices from Empty Slices
impact: MEDIUM
impactDescription: prevents encoding/reflection bugs
tags: slices, nil, empty, json
---

## Distinguish Nil Slices from Empty Slices

**Impact: MEDIUM (prevents encoding/reflection bugs)**

A nil slice (`var s []string`) and an empty slice (`s := []string{}`) both have length and capacity of zero, and both work with `len`, `cap`, `append`, and `range`. However, they differ in important ways: a nil slice equals `nil`, while an empty slice does not. Most critically, `json.Marshal` encodes a nil slice as `null` and an empty slice as `[]`. This distinction matters for API responses and any code that checks for `nil`.

Use `var s []string` (nil) when the final size is unknown and `nil` is acceptable. Use `[]string{}` or `make([]string, 0)` when you need an explicit empty collection, especially for JSON serialization.

**Incorrect (what's wrong):**

```go
var s []string
b, _ := json.Marshal(s) // "null"
```

**Correct (what's right):**

```go
s := []string{} // or make([]string, 0)
b, _ := json.Marshal(s) // "[]"

// Use var s []string when unsure about final length and nil is acceptable
```
