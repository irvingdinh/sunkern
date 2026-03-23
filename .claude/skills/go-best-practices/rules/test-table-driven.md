---
title: Use Table-Driven Tests
impact: MEDIUM
impactDescription: reduces test duplication
tags: testing, table-driven, subtests
---

## Use Table-Driven Tests

**Impact: MEDIUM (reduces test duplication)**

When testing the same function with multiple inputs, group cases into a table (slice of structs) and iterate with `t.Run`. This eliminates repetitive assertion code, makes adding new cases trivial, and gives each case a descriptive name that appears in test output on failure.

Each subtest created by `t.Run` can be run individually (`go test -run TestAdd/negative`), can fail independently, and can be parallelized with `t.Parallel()`.

**Incorrect (what's wrong):**

```go
func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Error("1+2 should be 3")
	}
	if Add(0, 0) != 0 {
		t.Error("0+0 should be 0")
	}
	if Add(-1, 1) != 0 {
		t.Error("-1+1 should be 0")
	}
	// Adding a new case means copy-pasting more boilerplate
	// If test 1 fails, you still see results for all — but messages are inconsistent
}
```

**Correct (what's right):**

```go
func TestAdd(t *testing.T) {
	tests := []struct {
		name string
		a, b int
		want int
	}{
		{"positive", 1, 2, 3},
		{"zeros", 0, 0, 0},
		{"negative", -1, 1, 0},
		{"large numbers", 1000000, 2000000, 3000000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Add(tt.a, tt.b); got != tt.want {
				t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
```
