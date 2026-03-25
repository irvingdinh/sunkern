---
title: Compare Errors with errors.Is and errors.As
impact: HIGH
impactDescription: prevents broken error checks with wrapping
tags: errors, comparison, is, as, wrapping
---

## Compare Errors with errors.Is and errors.As

**Impact: HIGH (prevents broken error checks with wrapping)**

Use errors.Is for sentinel values and errors.As for error types. Never use == or direct type assertions to check errors because they break when errors are wrapped with %w. Both errors.Is and errors.As unwrap the error chain recursively, so they work regardless of how many layers of wrapping exist between your code and the original error.

**Incorrect (what's wrong):**

```go
// Comparing sentinel value with == breaks if err is wrapped
if err == sql.ErrNoRows {
	// This fails when err is fmt.Errorf("query: %w", sql.ErrNoRows)
}

// Type assertion for error types breaks if err is wrapped
if _, ok := err.(*net.DNSError); ok {
	// This fails when err is fmt.Errorf("lookup: %w", dnsErr)
}
```

**Correct (what's right):**

```go
// errors.Is for sentinel values (unwraps recursively)
if errors.Is(err, sql.ErrNoRows) {
	// Works even if err is wrapped multiple times
}

// errors.As for error types (unwraps recursively)
var dnsErr *net.DNSError
if errors.As(err, &dnsErr) {
	fmt.Println(dnsErr.Name) // Access typed error fields
}
```
