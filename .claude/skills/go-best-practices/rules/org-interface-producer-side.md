---
title: Define Interfaces on the Consumer Side
impact: MEDIUM
impactDescription: Producer-side interfaces force all consumers to depend on the same abstraction
tags: interfaces, consumer, design
---

## Define Interfaces on the Consumer Side

**Impact: MEDIUM (Producer-side interfaces force all consumers to depend on the same abstraction)**

Interfaces should live on the consumer side, not the producer side. The client defines the abstraction it needs, keeping interfaces small and focused. This follows the Interface Segregation Principle and is idiomatic Go. The producer exports a concrete type; each consumer defines its own interface with only the methods it actually calls.

**Incorrect (what's wrong):**

```go
// package store — producer defines the interface
package store

type CustomerStorage interface {
    StoreCustomer(Customer) error
    GetCustomer(id string) (Customer, error)
    UpdateCustomer(Customer) error
    DeleteCustomer(id string) error
}

type Store struct{}

func (s *Store) StoreCustomer(c Customer) error  { /* ... */ return nil }
func (s *Store) GetCustomer(id string) (Customer, error) { /* ... */ return Customer{}, nil }
func (s *Store) UpdateCustomer(c Customer) error  { /* ... */ return nil }
func (s *Store) DeleteCustomer(id string) error   { /* ... */ return nil }
```

**Correct (what's right):**

```go
// package store — producer exports only the concrete type
package store

type Store struct{}

func (s *Store) StoreCustomer(c Customer) error  { /* ... */ return nil }
func (s *Store) GetCustomer(id string) (Customer, error) { /* ... */ return Customer{}, nil }
func (s *Store) UpdateCustomer(c Customer) error  { /* ... */ return nil }
func (s *Store) DeleteCustomer(id string) error   { /* ... */ return nil }

// package billing — consumer defines only what it needs
package billing

type customerStorer interface {
    StoreCustomer(store.Customer) error
}

type Service struct {
    storer customerStorer
}
```

Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #6.
