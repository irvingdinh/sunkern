---
title: Understand Defer Argument Evaluation
impact: HIGH
impactDescription: prevents using stale values in defer
tags: defer, evaluation, closure, arguments
---

## Understand Defer Argument Evaluation

**Impact: HIGH (prevents using stale values in defer)**

Defer evaluates function arguments IMMEDIATELY at the defer statement, not when the deferred function executes. This means passing a variable by value captures its current value, not its final value. Use closures to capture variables by reference, or pass a pointer if the function signature allows it.

**Incorrect (what's wrong):**

```go
func f() error {
	var status string
	defer notify(status)           // status is "" at this point
	defer incrementCounter(status) // status is "" at this point
	if err := foo(); err != nil {
		status = "error"
		return err
	}
	status = "success"
	return nil
}
// notify and incrementCounter always receive ""
```

**Correct (what's right):**

```go
func f() error {
	var status string
	defer func() {
		notify(status)           // Captures status by reference
		incrementCounter(status) // Gets the final value of status
	}()
	if err := foo(); err != nil {
		status = "error"
		return err
	}
	status = "success"
	return nil
}
// Or pass a pointer: defer notify(&status) if notify accepts *string
```
