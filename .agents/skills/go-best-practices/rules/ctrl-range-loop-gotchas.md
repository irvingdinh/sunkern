---
title: Range Loop Gotchas — Value Copies and Expression Evaluation
impact: HIGH
impactDescription: prevents silent value copy bugs
tags: range, loops, copy, evaluation
---

## Range Loop Gotchas — Value Copies and Expression Evaluation

**Impact: HIGH (prevents silent value copy bugs)**

The range loop has two subtle behaviors that cause silent bugs. First, the value element is always a copy of the original element. Mutating it has no effect on the underlying collection. Second, the range expression is evaluated once before the loop begins. For arrays (not slices), this means Go copies the entire array, and modifications to the original array during iteration are invisible to the loop.

**Incorrect (mutating a copy has no effect):**

```go
type Account struct{ balance float64 }

accounts := []Account{{100}, {200}, {300}}
for _, a := range accounts {
	a.balance += 10 // Mutates the copy, not the original
}
// accounts unchanged: [{100} {200} {300}]
```

**Correct (index into the slice directly):**

```go
for i := range accounts {
	accounts[i].balance += 10
}
```

**Incorrect (range expression evaluated once — array is copied):**

```go
a := [3]int{0, 1, 2}
for i, v := range a {
	a[2] = 10
	if i == 2 {
		fmt.Println(v) // Prints 2, not 10 — range copied the array
	}
}
```

**Correct (range over a slice to avoid the copy):**

```go
a := [3]int{0, 1, 2}
for i, v := range a[:] { // Range over slice — no copy
	a[2] = 10
	if i == 2 {
		fmt.Println(v) // Prints 10
	}
}
```
