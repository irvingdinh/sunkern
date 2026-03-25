---
title: Beware Named Result Zero-Value Side Effects
impact: MEDIUM
impactDescription: prevents returning zero-value errors
tags: named-results, zero-value, bugs
---

## Beware Named Result Zero-Value Side Effects

**Impact: MEDIUM (prevents returning zero-value errors)**

Named result parameters are initialized to their zero values. When a named error variable is never assigned, returning it returns nil instead of the intended error. This creates a silent bug where the caller believes the operation succeeded. Always return explicit error values rather than relying on named error variables that may not have been set.

**Incorrect (what's wrong):**

```go
func getCoordinates(ctx context.Context, address string) (lat, lng float32, err error) {
	if ctx.Err() != nil {
		return 0, 0, err // Bug: err is nil (zero value), not ctx.Err()
	}
	// ...
	return lat, lng, nil
}
```

**Correct (what's right):**

```go
func getCoordinates(ctx context.Context, address string) (lat, lng float32, err error) {
	if ctx.Err() != nil {
		return 0, 0, ctx.Err() // Explicit error value
	}
	// ...
	return lat, lng, nil
}
```
