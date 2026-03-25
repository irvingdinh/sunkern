---
title: Avoid Interface Pollution
impact: HIGH
impactDescription: Premature interfaces add indirection and complexity without real benefit
tags: interfaces, abstraction, design
---

## Avoid Interface Pollution

**Impact: HIGH (Premature interfaces add indirection and complexity without real benefit)**

Don't create interfaces before you need them. Abstractions should be discovered, not created up front. If it's unclear how an interface makes the code better — easier to test, easier to extend, decoupled from implementation — don't add it. An interface with a single implementation and no testing need is almost always premature.

**Incorrect (what's wrong):**

```go
// Premature interface — only one implementation exists
type Store interface {
    GetUser(id string) (User, error)
    SaveUser(u User) error
}

type InMemoryStore struct {
    users map[string]User
}

func (s *InMemoryStore) GetUser(id string) (User, error) {
    u, ok := s.users[id]
    if !ok {
        return User{}, fmt.Errorf("user %s not found", id)
    }
    return u, nil
}

func (s *InMemoryStore) SaveUser(u User) error {
    s.users[u.ID] = u
    return nil
}
```

**Correct (what's right):**

```go
// Start with a concrete type
type InMemoryStore struct {
    users map[string]User
}

func (s *InMemoryStore) GetUser(id string) (User, error) {
    u, ok := s.users[id]
    if !ok {
        return User{}, fmt.Errorf("user %s not found", id)
    }
    return u, nil
}

func (s *InMemoryStore) SaveUser(u User) error {
    s.users[u.ID] = u
    return nil
}

// Create an interface only when a second implementation or testing requires it
```

Three valid reasons to use an interface: common behavior (e.g., `io.Reader`), decoupling (consumer doesn't need to know the implementation), restricting behavior (exposing a subset of methods). Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #5.
