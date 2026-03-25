---
title: Use Go Testing Toolchain Features
impact: LOW-MEDIUM
impactDescription: utilizes Go testing toolchain
tags: testing, coverage, setup, teardown
---

## Use Go Testing Toolchain Features

**Impact: LOW-MEDIUM (utilizes Go testing toolchain)**

Go's testing package has built-in features that many developers overlook:

- **`-coverprofile`**: Generate coverage reports to identify untested code paths.
- **External test packages (`_test`)**: Use `package foo_test` to write black-box tests that can only access the exported API, catching accidentally leaked internals.
- **`t.Helper()`**: Mark utility functions as test helpers so error messages point to the caller, not the helper.
- **`TestMain`**: Run setup and teardown logic once for the entire test file (e.g., start a database, load fixtures).

**Incorrect (what's wrong):**

```go
// Helper without t.Helper() — errors point to the wrong line
func assertEqual(t *testing.T, got, want int) {
	if got != want {
		t.Errorf("got %d, want %d", got, want) // Error reported here, not at caller
	}
}

func TestSomething(t *testing.T) {
	assertEqual(t, compute(), 42) // Line 25 — but error says line 4
}

// Setup/teardown duplicated in every test
func TestA(t *testing.T) {
	db := setupDB()
	defer teardownDB(db)
	// test...
}

func TestB(t *testing.T) {
	db := setupDB()
	defer teardownDB(db)
	// test...
}
```

**Correct (what's right):**

```go
// t.Helper() makes errors point to the caller
func assertEqual(t *testing.T, got, want int) {
	t.Helper() // Error now points to the caller
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestSomething(t *testing.T) {
	assertEqual(t, compute(), 42) // Error correctly reported at this line
}

// TestMain for shared setup/teardown
var testDB *sql.DB

func TestMain(m *testing.M) {
	// Setup
	testDB = setupDB()

	// Run all tests
	code := m.Run()

	// Teardown
	teardownDB(testDB)
	os.Exit(code)
}

// Black-box testing with external test package
// File: parser_test.go
package parser_test // Can only use exported API

import "myapp/parser"

func TestParse(t *testing.T) {
	result, err := parser.Parse("input") // Tests the public interface
	// ...
}

// Generate coverage: go test -coverprofile=coverage.out ./...
// View in browser: go tool cover -html=coverage.out
```
