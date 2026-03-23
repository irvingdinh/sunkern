---
title: Context Usage for Cancellation and Deadlines
impact: HIGH
impactDescription: enables cancellation and deadlines
tags: context, deadline, cancellation, values
---

## Context Usage for Cancellation and Deadlines

**Impact: HIGH (enables cancellation and deadlines)**

Context carries deadline, cancellation signal, and key-value pairs. Functions users wait for should accept context. The Done channel is closed on cancellation.

**Incorrect (what's wrong):**

```go
func fetchData() (Data, error) {
    // No way to cancel or timeout this operation
    return http.Get("https://api.example.com/data")
}
```

**Correct (what's right):**

```go
func fetchData(ctx context.Context) (Data, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", "https://api.example.com/data", nil)
    if err != nil { return Data{}, err }
    return http.DefaultClient.Do(req)
}
```
