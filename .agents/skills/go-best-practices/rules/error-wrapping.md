---
title: Wrap Errors with Intent
impact: HIGH
impactDescription: controls error chain visibility
tags: errors, wrapping, fmt, context
---

## Wrap Errors with Intent

**Impact: HIGH (controls error chain visibility)**

Wrap with %w to add context AND let callers inspect the source error using errors.Is and errors.As. Use %v or create a new error to add context but HIDE the source error, preventing callers from coupling to internal implementation details. The choice between %w and %v is an API design decision: %w makes the wrapped error part of your public contract, while %v keeps it private.

**Incorrect (what's wrong):**

```go
// Using %v when callers need to inspect the cause — loses error chain
if err != nil {
	return fmt.Errorf("failed: %v", err) // errors.Is/As won't work on the result
}

// Using %w when exposing internal errors creates unwanted coupling
if err != nil {
	return fmt.Errorf("query failed: %w", err) // Callers can now match on sql.ErrNoRows
}
```

**Correct (what's right):**

```go
// When callers SHOULD inspect the cause (error is part of your API):
return fmt.Errorf("getting user %s: %w", id, err)

// When callers should NOT know the cause (implementation detail):
return fmt.Errorf("getting user %s: %v", id, err)
```
