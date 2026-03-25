---
title: Substring References Leak the Original String
impact: MEDIUM
impactDescription: prevents memory leaks
tags: strings, substring, memory-leak, copy
---

## Substring References Leak the Original String

**Impact: MEDIUM (prevents memory leaks)**

A substring created with s[a:b] shares the same backing byte array as the original string. As long as the substring is reachable, the garbage collector cannot free the original string, even if only a tiny portion is needed. For functions that extract a small piece from a large string (such as a UUID from a multi-kilobyte payload), this keeps the entire original allocation alive indefinitely.

**Incorrect (what's wrong):**

```go
func extractID(large string) string {
	return large[:36] // Shares backing array — entire large string stays in memory
}
```

**Correct (what's right):**

```go
func extractID(large string) string {
	return strings.Clone(large[:36]) // Go 1.20+ — creates independent copy
	// Or: return string([]byte(large[:36]))
}
```
