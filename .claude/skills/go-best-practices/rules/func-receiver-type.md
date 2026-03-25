---
title: Choose Receiver Type Deliberately
impact: HIGH
impactDescription: affects mutability and performance
tags: receiver, pointer, value, methods
---

## Choose Receiver Type Deliberately

**Impact: HIGH (affects mutability and performance)**

MUST use a pointer receiver if the method mutates the receiver or the receiver contains non-copyable fields such as sync.Mutex. SHOULD use a pointer receiver when the receiver is a large struct. MUST use a value receiver to enforce immutability or when the receiver is a map, function, or channel. SHOULD use a value receiver for small structs, basic types, and slices that do not need mutation. Mixing receiver types on the same type is a code smell unless there is a clear reason.

**Incorrect (what's wrong):**

```go
type Customer struct {
	name  string
	mutex sync.Mutex
}

func (c Customer) UpdateName(name string) { // Value receiver can't mutate, and copies sync.Mutex
	c.name = name // This change is lost
}
```

**Correct (what's right):**

```go
type Customer struct {
	name  string
	mutex sync.Mutex
}

func (c *Customer) UpdateName(name string) {
	c.name = name // Mutates the original
}
```
