---
title: Guard Against Integer Overflow
impact: HIGH
impactDescription: prevents silent arithmetic bugs
tags: integers, overflow, safety
---

## Guard Against Integer Overflow

**Impact: HIGH (prevents silent arithmetic bugs)**

Go catches integer overflow at compile time for constant expressions, but at runtime, integer overflow wraps silently. For signed integers, this means adding two large positive numbers can produce a negative result. For unsigned integers, subtracting below zero wraps to the maximum value. This silent wrapping can lead to subtle, hard-to-diagnose bugs in counters, accumulators, and size calculations.

Always validate arithmetic operations when working with values that may approach type boundaries.

**Incorrect (what's wrong):**

```go
var counter int32 = math.MaxInt32
counter++ // Silently wraps to -2147483648
```

**Correct (what's right):**

```go
func safeAdd32(a, b int32) (int32, error) {
	if (b > 0 && a > math.MaxInt32-b) || (b < 0 && a < math.MinInt32-b) {
		return 0, fmt.Errorf("integer overflow: %d + %d", a, b)
	}
	return a + b, nil
}
```
