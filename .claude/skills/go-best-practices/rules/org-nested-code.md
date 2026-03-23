---
title: Avoid Deeply Nested Code
impact: MEDIUM
impactDescription: Deep nesting reduces readability and makes the happy path hard to follow
tags: readability, nesting, happy-path
---

## Avoid Deeply Nested Code

**Impact: MEDIUM (Deep nesting reduces readability and makes the happy path hard to follow)**

Keep the happy path left-aligned. When an `if` block ends with a `return`, omit the `else`. Flip conditions to return early for error cases rather than wrapping the main logic in a deeply nested block. This aligns with the Go proverb: "the happy path is un-indented."

**Incorrect (what's wrong):**

```go
func process(s string) error {
    if s != "" {
        if len(s) < 100 {
            if isValid(s) {
                // ... lots of processing code
                return nil
            } else {
                return errors.New("invalid string")
            }
        } else {
            return errors.New("string too long")
        }
    } else {
        return errors.New("empty string")
    }
}
```

**Correct (what's right):**

```go
func process(s string) error {
    if s == "" {
        return errors.New("empty string")
    }
    if len(s) >= 100 {
        return errors.New("string too long")
    }
    if !isValid(s) {
        return errors.New("invalid string")
    }

    // ... lots of processing code
    return nil
}
```

Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #2.
