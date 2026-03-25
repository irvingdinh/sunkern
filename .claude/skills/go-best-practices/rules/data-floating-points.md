---
title: Handle Floating-Point Arithmetic Carefully
impact: MEDIUM
impactDescription: prevents accuracy bugs
tags: floating-point, accuracy, comparison
---

## Handle Floating-Point Arithmetic Carefully

**Impact: MEDIUM (prevents accuracy bugs)**

Floating-point numbers (float32, float64) use IEEE 754 representation and cannot represent all decimal values exactly. Arithmetic on floats accumulates rounding errors, making direct equality comparisons unreliable. To mitigate precision loss: compare floats within a small delta (epsilon), group additions by magnitude (add small numbers together before adding to large ones), and prefer performing multiplication and division before addition and subtraction.

**Incorrect (what's wrong):**

```go
var n float32 = 1.0001
fmt.Println(n * n) // Prints 1.0002, not 1.00020001

result := f1 == f2 // Direct comparison — unreliable for computed floats
```

**Correct (what's right):**

```go
const epsilon = 1e-7

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}
```
