---
title: Handle Each Error Exactly Once
impact: HIGH
impactDescription: prevents duplicated or lost errors
tags: errors, handling, logging, defer
---

## Handle Each Error Exactly Once

**Impact: HIGH (prevents duplicated or lost errors)**

Three rules for disciplined error handling. (A) Handle an error ONCE. Logging IS handling. Either log the error or return it to the caller, never both -- doing both produces duplicate log entries and confuses the error's origin. (B) If you intentionally ignore an error, assign it to the blank identifier to make the intent explicit. (C) Handle errors from deferred calls -- do not silently drop them, especially for writes and Close on writable resources.

**Incorrect (what's wrong):**

```go
// (A) Handling twice: logging AND returning
func getUser(id string) (User, error) {
	user, err := db.FindUser(id)
	if err != nil {
		log.Printf("error finding user: %v", err) // Handles once (logs)
		return User{}, err                          // Handles again (returns) — duplicate
	}
	return user, nil
}

// (B) Silently ignoring an error — was this intentional?
notify()

// (C) Defer error lost
defer file.Close() // Error silently dropped
```

**Correct (what's right):**

```go
// (A) Handle once: return with context, let the caller decide to log
func getUser(id string) (User, error) {
	user, err := db.FindUser(id)
	if err != nil {
		return User{}, fmt.Errorf("finding user %s: %w", id, err)
	}
	return user, nil
}

// (B) Explicitly ignored error
_ = notify()

// (C) Defer error propagated using named return
defer func() {
	if closeErr := file.Close(); closeErr != nil {
		err = errors.Join(err, closeErr) // Propagate using named return
	}
}()
```
