---
title: Avoid Init Functions
impact: HIGH
impactDescription: Init functions limit error handling, complicate testing, and force global state
tags: init, initialization, testing
---

## Avoid Init Functions

**Impact: HIGH (Init functions limit error handling, complicate testing, and force global state)**

Init functions run automatically when a package is loaded, before `main()`. They cannot return errors, so failures must be handled with `log.Fatal` or `panic`, making them unrecoverable. They also require global mutable state, which complicates testing and makes dependencies implicit. Use explicit initialization functions that return errors instead.

**Incorrect (what's wrong):**

```go
var db *sql.DB

func init() {
    d, err := sql.Open("postgres", dsn)
    if err != nil {
        log.Fatal(err)
    }
    db = d
}
```

**Correct (what's right):**

```go
func NewDB(dsn string) (*sql.DB, error) {
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, fmt.Errorf("opening db: %w", err)
    }
    return db, nil
}
```

The only acceptable uses of `init()` are side-effect-free operations like initializing a static map or registering a driver via a blank import. Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #3.
