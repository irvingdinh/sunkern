---
title: Use Named Result Parameters for Clarity
impact: LOW-MEDIUM
impactDescription: improves readability when used well
tags: named-results, readability, return
---

## Use Named Result Parameters for Clarity

**Impact: LOW-MEDIUM (improves readability when used well)**

Named result parameters improve readability when multiple parameters share a type, making it clear which value is which. They are initialized to their zero value and enable naked returns. Use them sparingly and primarily for documentation purposes. Avoid naked returns in long functions as they hurt readability.

**Incorrect (what's wrong):**

```go
func ConvertCoords(lat, lng float64) (float64, float64) { // Which is x? Which is y?
	// conversion logic
	return x, y
}
```

**Correct (what's right):**

```go
func ConvertCoords(lat, lng float64) (x, y float64) { // Clear what each return value represents
	// conversion logic
	return x, y
}
```
