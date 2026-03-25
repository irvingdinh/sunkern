---
title: Use Race Detection, Parallel, and Shuffle Modes
impact: MEDIUM
impactDescription: catches races and order dependencies
tags: testing, race, parallel, shuffle
---

## Use Race Detection, Parallel, and Shuffle Modes

**Impact: MEDIUM (catches races and order dependencies)**

Go's test runner supports several execution modes that catch different classes of bugs:

- **`-race`**: Detects data races at runtime. Data races are among the hardest concurrency bugs to find and can cause silent corruption. Always run with `-race` in CI.
- **`-parallel`**: Controls how many tests marked with `t.Parallel()` run concurrently. Speeds up test suites and can expose concurrency issues.
- **`-shuffle`**: Randomizes test execution order to catch tests that depend on the order of other tests (e.g., shared global state).
- **Build tags**: Categorize tests (unit, integration, e2e) so they can be run selectively.

**Incorrect (what's wrong):**

```go
// Running tests without race detection misses data races
// go test ./...

// Tests pass only because they run in a specific order
func TestA(t *testing.T) {
	globalState = "initialized"
}

func TestB(t *testing.T) {
	// Depends on TestA running first — breaks with -shuffle
	if globalState != "initialized" {
		t.Fatal("not initialized")
	}
}
```

**Correct (what's right):**

```go
// Run with race detection, shuffled order, and parallelism:
// go test -race -shuffle=on -parallel=4 ./...

// Each test is self-contained
func TestA(t *testing.T) {
	state := setup() // Own setup, no shared mutable state
	defer cleanup(state)
	// ...
}

func TestB(t *testing.T) {
	state := setup() // Own setup, no order dependency
	defer cleanup(state)
	// ...
}

// Use build tags to categorize tests:
//go:build integration

package mypackage_test

func TestDatabaseIntegration(t *testing.T) {
	// Only runs with: go test -tags=integration ./...
}
```
