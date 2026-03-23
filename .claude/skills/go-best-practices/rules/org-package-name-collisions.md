---
title: Avoid Package Name Collisions with Variables
impact: LOW
impactDescription: Naming a variable after an imported package prevents using that package in scope
tags: naming, packages, collisions
---

## Avoid Package Name Collisions with Variables

**Impact: LOW (Naming a variable after an imported package prevents using that package in scope)**

Don't name variables the same as imported packages. When a variable shadows a package name, you can no longer use that package in the same scope. This commonly happens with packages like `context`, `http`, `errors`, and `strings`.

**Incorrect (what's wrong):**

```go
func handler(ctx context.Context) error {
    context := extractContext(ctx) // shadows the "context" package
    // Can no longer call context.WithTimeout, context.WithCancel, etc.

    http := performRequest(context)
    // Can no longer call http.Get, http.NewRequest, etc.

    _ = http
    return nil
}
```

**Correct (what's right):**

```go
func handler(ctx context.Context) error {
    reqCtx := extractContext(ctx)

    resp := performRequest(reqCtx)

    // Can still use both packages freely
    newCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(newCtx, http.MethodGet, "/api", nil)
    if err != nil {
        return err
    }

    _ = resp
    _ = req
    return nil
}
```

Choose variable names that describe their purpose rather than their type: `reqCtx` instead of `context`, `resp` instead of `http`. Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #14.
