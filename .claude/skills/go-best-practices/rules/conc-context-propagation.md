---
title: Context Propagation in Async Goroutines
impact: HIGH
impactDescription: prevents premature cancellation
tags: context, propagation, http, goroutines
---

## Context Propagation in Async Goroutines

**Impact: HIGH (prevents premature cancellation)**

HTTP request context cancels when response is written. Don't propagate it to async goroutines that must outlive the request.

**Incorrect (what's wrong):**

```go
func handler(w http.ResponseWriter, r *http.Request) {
    response := doWork(r.Context())
    go func() {
        publish(r.Context(), response) // Context cancels after response is written
    }()
    writeResponse(w, response)
}
```

**Correct (what's right):**

```go
func handler(w http.ResponseWriter, r *http.Request) {
    response := doWork(r.Context())
    go func() {
        ctx := context.WithoutCancel(r.Context()) // Go 1.21+
        publish(ctx, response)
    }()
    writeResponse(w, response)
}
```
