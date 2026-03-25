---
title: Organize Project Structure Thoughtfully
impact: LOW-MEDIUM
impactDescription: Poor package structure causes tight coupling, circular imports, and discoverability issues
tags: project, packages, organization
---

## Organize Project Structure Thoughtfully

**Impact: LOW-MEDIUM (Poor package structure causes tight coupling, circular imports, and discoverability issues)**

Avoid premature packaging. Don't create dozens of nano packages with one or two files each, and don't create monolithic packages with hundreds of files. Organize by context (domain-driven) or by layer (technical role) — either approach works, but be consistent. Minimize exports: only export what other packages actually need.

**Incorrect (what's wrong):**

```go
// Scattered nano packages with vague names
// myapp/common/types.go
package common

type User struct { /* ... */ }

// myapp/util/helpers.go
package util

func FormatName(first, last string) string { /* ... */ return "" }

// myapp/shared/constants.go
package shared

const MaxRetries = 3

// myapp/base/base.go
package base

type BaseService struct { /* ... */ }

// myapp/models/models.go
package models

type Order struct { /* ... */ }
```

**Correct (what's right):**

```go
// Packages named after what they provide, consistent granularity

// myapp/customer/customer.go
package customer

type Customer struct {
    ID   string
    Name string
}

func (c *Customer) FullName() string { return c.Name }

// myapp/order/order.go
package order

type Order struct {
    ID         string
    CustomerID string
    Items      []Item
}

// myapp/order/item.go
package order

type Item struct {
    ProductID string
    Quantity  int
}

// myapp/http/handler.go
package http

type Handler struct {
    customers *customer.Service
    orders    *order.Service
}
```

Guidelines: keep the number of packages proportional to the project size, avoid `common/` and `util/` (see the utility packages rule), and start with fewer packages — split when a package grows too large or has too many responsibilities. Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #12.
