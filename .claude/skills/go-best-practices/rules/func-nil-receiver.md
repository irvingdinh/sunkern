---
title: Never Return a Typed Nil as an Interface
impact: HIGH
impactDescription: prevents non-nil interface with nil value
tags: interfaces, nil, receiver, type-assertion
---

## Never Return a Typed Nil as an Interface

**Impact: HIGH (prevents non-nil interface with nil value)**

Returning a nil pointer typed as an interface produces a non-nil interface value. An interface in Go is a pair of (type, value). When a nil pointer of a concrete type is assigned to an interface, the interface holds (type=*ConcreteType, value=nil), which is not equal to nil. Callers checking err != nil will see it as non-nil, leading to false positive error handling. Always return an explicit untyped nil for the success path.

**Incorrect (what's wrong):**

```go
func validate(s string) error {
	var err *ValidationError // nil pointer
	if s == "" {
		err = &ValidationError{Field: "s"}
	}
	return err // Returns non-nil interface even when err pointer is nil
}
// validate("hello") != nil is TRUE — unexpected
```

**Correct (what's right):**

```go
func validate(s string) error {
	if s == "" {
		return &ValidationError{Field: "s"}
	}
	return nil // Return explicit nil
}
// validate("hello") == nil is TRUE — expected
```
