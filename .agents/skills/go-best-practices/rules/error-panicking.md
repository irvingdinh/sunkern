---
title: Reserve Panic for Programmer Errors
impact: MEDIUM
impactDescription: reserve for unrecoverable errors
tags: panic, errors, recovery
---

## Reserve Panic for Programmer Errors

**Impact: MEDIUM (reserve for unrecoverable errors)**

Panic only for programmer errors such as a nil driver passed to sql.Register or when a mandatory dependency fails to initialize at startup. Never panic for recoverable runtime conditions like database query failures, network timeouts, or invalid user input. These must be returned as errors so the caller can decide how to handle them. A panic in a running server kills the entire process and all in-flight requests.

**Incorrect (what's wrong):**

```go
func getUser(id string) User {
	user, err := db.FindUser(id)
	if err != nil {
		panic(err) // Kills the entire application for a query error
	}
	return user
}
```

**Correct (what's right):**

```go
func getUser(id string) (User, error) {
	user, err := db.FindUser(id)
	if err != nil {
		return User{}, fmt.Errorf("finding user %s: %w", id, err)
	}
	return user, nil
}

// Acceptable panic: programmer error caught at init time
func init() {
	sql.Register("custom", nil) // Panics if driver is nil — this is correct
}
```
