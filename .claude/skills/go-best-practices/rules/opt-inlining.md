---
title: Use Fast-Path Inlining
impact: LOW-MEDIUM
impactDescription: reduces function call overhead
tags: inlining, performance, fast-path
---

## Use Fast-Path Inlining

**Impact: LOW-MEDIUM (reduces function call overhead)**

Go automatically inlines small functions (those with a low "inline cost"). Inlined functions avoid the overhead of a function call: no stack frame setup, no register saving, and the inlined code can be further optimized by the compiler in context.

The fast-path inlining pattern exploits this: write a small wrapper function that handles the common (fast) case, and call a separate larger function for the uncommon (slow) case. The wrapper stays small enough to be inlined, so the fast path has zero call overhead.

Use `go build -gcflags="-m"` to see which functions are inlined and which are not.

**Incorrect (what's wrong):**

```go
// Too large to inline — every call pays function call overhead
func isValid(s string) bool {
	if s == "" {
		return false
	}
	// Complex validation logic pushes inline cost too high
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	if len(s) > 255 {
		return false
	}
	return true
}

// Every call to isValid pays ~5ns function call overhead
// even when s is empty (the most common rejection case)
```

**Correct (what's right):**

```go
// Small wrapper — inlined by the compiler
func isValid(s string) bool {
	if s == "" {
		return false // Fast path: handled inline, no call overhead
	}
	return isValidSlow(s) // Slow path: only called when needed
}

// Complex logic in a separate function
func isValidSlow(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	if len(s) > 255 {
		return false
	}
	return true
}

// Verify inlining:
// go build -gcflags="-m" ./...
// ./validator.go:4:6: can inline isValid
// ./validator.go:11:6: cannot inline isValidSlow

// Real-world example from sync.Mutex:
// Lock is a tiny function that tries a fast CAS.
// If contended, it calls lockSlow which handles queuing.
```
