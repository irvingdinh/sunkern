---
title: Avoid Variable Shadowing
impact: MEDIUM
impactDescription: Shadowed variables cause silent bugs where outer values remain unchanged
tags: variables, shadowing, scope
---

## Avoid Variable Shadowing

**Impact: MEDIUM (Shadowed variables cause silent bugs where outer values remain unchanged)**

Variable shadowing occurs when a variable name is redeclared in an inner block using `:=`. The inner variable hides the outer one silently, meaning assignments to the inner variable do not affect the outer one. This is a common source of bugs, especially with `err`, where an error may be silently swallowed because the outer `err` remains `nil`.

**Incorrect (what's wrong):**

```go
var client *http.Client
if tracing {
    client, err := createClientWithTracing() // err is shadowed, outer err unchanged
    if err != nil {
        return err
    }
    log.Println(client)
} else {
    client, err := createDefaultClient() // same problem
    if err != nil {
        return err
    }
    log.Println(client)
}
```

**Correct (what's right):**

```go
var client *http.Client
var err error
if tracing {
    client, err = createClientWithTracing()
} else {
    client, err = createDefaultClient()
}
if err != nil {
    return err
}
```

Use `go vet -vettool=$(which shadow)` or enable the `shadow` linter in golangci-lint to detect variable shadowing at build time. Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #1.
