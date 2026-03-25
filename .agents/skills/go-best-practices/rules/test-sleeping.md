---
title: Never Use time.Sleep for Test Synchronization
impact: MEDIUM
impactDescription: prevents flaky tests
tags: testing, sleep, synchronization, flaky
---

## Never Use time.Sleep for Test Synchronization

**Impact: MEDIUM (prevents flaky tests)**

Using `time.Sleep` to wait for asynchronous operations in tests creates flaky tests. The sleep is either too short (test fails intermittently on slow CI machines) or too long (test suite is unnecessarily slow). Both are bad.

Instead, use proper synchronization: channels, `sync.WaitGroup`, condition variables, or a retry loop with a timeout. These make tests both reliable and fast -- they proceed as soon as the condition is met, with a hard timeout as a safety net.

**Incorrect (what's wrong):**

```go
func TestAsync(t *testing.T) {
	go produce()
	time.Sleep(500 * time.Millisecond) // Flaky — may be too short or too long
	assertResult(t)
}

func TestEventualConsistency(t *testing.T) {
	triggerUpdate()
	time.Sleep(2 * time.Second) // Wastes 2 seconds even if ready in 10ms
	checkState(t)
}
```

**Correct (what's right):**

```go
// Channel-based synchronization
func TestAsync(t *testing.T) {
	done := make(chan struct{})
	go func() {
		produce()
		close(done)
	}()
	select {
	case <-done:
		assertResult(t)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for produce")
	}
}

// Retry with timeout for eventually-consistent operations
func TestEventualConsistency(t *testing.T) {
	triggerUpdate()

	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for state update")
		case <-ticker.C:
			if stateIsReady() {
				checkState(t)
				return
			}
		}
	}
}
```
