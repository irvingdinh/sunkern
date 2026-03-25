---
title: Break Terminates the Innermost Statement
impact: MEDIUM
impactDescription: prevents breaking wrong statement
tags: break, switch, select, labels
---

## Break Terminates the Innermost Statement

**Impact: MEDIUM (prevents breaking wrong statement)**

A break statement in Go terminates the innermost for, switch, or select statement, not necessarily the construct you intended. When a switch or select is nested inside a for loop, break only exits the switch or select, leaving the loop running. Use a labeled break to target the outer loop explicitly.

**Incorrect (what's wrong):**

```go
for i := 0; i < 5; i++ {
	switch i {
	case 2:
		break // Breaks the switch, not the for loop
	}
}
// Iterates 0..4, break has no effect on loop
```

**Correct (what's right):**

```go
loop:
	for i := 0; i < 5; i++ {
		switch i {
		case 2:
			break loop // Breaks the for loop
		}
	}
// Iterates 0..2
```
