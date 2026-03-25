---
title: Return Concrete Types, Accept Interfaces
impact: MEDIUM
impactDescription: Returning interfaces forces all callers to depend on the same abstraction
tags: interfaces, return-types, flexibility
---

## Return Concrete Types, Accept Interfaces

**Impact: MEDIUM (Returning interfaces forces all callers to depend on the same abstraction)**

Functions should return concrete types and accept interfaces. Returning an interface restricts what callers can do with the result and couples them to that specific abstraction. Returning a concrete type gives callers maximum flexibility — they can use it directly or assign it to any interface it satisfies.

**Incorrect (what's wrong):**

```go
type Store struct {
    db *sql.DB
}

type StoreInterface interface {
    GetUser(id string) (User, error)
    SaveUser(u User) error
}

// Returns interface — callers cannot access Store-specific methods
func NewStore(db *sql.DB) StoreInterface {
    return &Store{db: db}
}
```

**Correct (what's right):**

```go
type Store struct {
    db *sql.DB
}

func (s *Store) GetUser(id string) (User, error) { /* ... */ return User{}, nil }
func (s *Store) SaveUser(u User) error            { /* ... */ return nil }

// Returns concrete type — callers can use directly or assign to any interface
func NewStore(db *sql.DB) *Store {
    return &Store{db: db}
}
```

The exception is when a function genuinely can return multiple concrete types (e.g., `errors.New` returns the `error` interface). Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #7.
