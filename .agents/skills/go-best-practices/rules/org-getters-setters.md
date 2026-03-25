---
title: Use Getters and Setters Judiciously
impact: LOW
impactDescription: Unnecessary getters and setters add clutter without providing value
tags: getters, setters, idiom
---

## Use Getters and Setters Judiciously

**Impact: LOW (Unnecessary getters and setters add clutter without providing value)**

Go does not require getters and setters. Using them when they add no value — no validation, no computed logic, no future compatibility concern — just adds noise. When you do need a getter, the Go convention is to drop the `Get` prefix: use `Balance()`, not `GetBalance()`. Export the field directly when no encapsulation logic is needed.

**Incorrect (what's wrong):**

```go
type Customer struct {
    balance float64
}

func (c *Customer) GetBalance() float64 {
    return c.balance
}

func (c *Customer) SetBalance(b float64) {
    c.balance = b
}
```

**Correct (what's right):**

```go
// Option A: exported field when no logic is needed
type Customer struct {
    Balance float64
}

// Option B: getter without Get prefix when encapsulation is needed
type Customer struct {
    balance float64
}

func (c *Customer) Balance() float64 {
    return c.balance
}

func (c *Customer) SetBalance(b float64) {
    if b < 0 {
        panic("balance cannot be negative")
    }
    c.balance = b
}
```

Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #4.
