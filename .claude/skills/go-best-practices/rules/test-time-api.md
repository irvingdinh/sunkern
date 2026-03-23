---
title: Inject Time as a Dependency
impact: MEDIUM
impactDescription: makes time-dependent code testable
tags: testing, time, dependency-injection
---

## Inject Time as a Dependency

**Impact: MEDIUM (makes time-dependent code testable)**

Code that calls `time.Now()` directly is impossible to test deterministically. Expiry checks, rate limiters, scheduling logic, and cache TTLs all become untestable because you cannot control the current time.

Instead, inject time as a dependency -- either as a function `func() time.Time` or as an interface. In production, pass `time.Now`. In tests, pass a function that returns a fixed or controllable time.

**Incorrect (what's wrong):**

```go
func isExpired(token Token) bool {
	return time.Now().After(token.ExpiresAt) // Untestable — can't control time.Now()
}

func (c *Cache) Get(key string) (string, bool) {
	entry, ok := c.data[key]
	if !ok {
		return "", false
	}
	if time.Now().After(entry.ExpiresAt) { // Untestable TTL logic
		delete(c.data, key)
		return "", false
	}
	return entry.Value, true
}
```

**Correct (what's right):**

```go
// Define a Clock type
type Clock func() time.Time

func isExpired(token Token, now Clock) bool {
	return now().After(token.ExpiresAt)
}

// Production: isExpired(token, time.Now)
// Test:
func TestIsExpired(t *testing.T) {
	fixedTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	token := Token{ExpiresAt: fixedTime.Add(-1 * time.Hour)}
	if !isExpired(token, clock) {
		t.Error("expected token to be expired")
	}

	token = Token{ExpiresAt: fixedTime.Add(1 * time.Hour)}
	if isExpired(token, clock) {
		t.Error("expected token to be valid")
	}
}

// For structs, inject via field
type Cache struct {
	data map[string]entry
	now  Clock
}

func NewCache(now Clock) *Cache {
	return &Cache{data: make(map[string]entry), now: now}
}
```
