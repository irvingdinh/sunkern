---
title: Defer in a Loop Delays Cleanup Until Function Returns
impact: HIGH
impactDescription: prevents resource leaks
tags: defer, loops, resources, file-descriptors
---

## Defer in a Loop Delays Cleanup Until Function Returns

**Impact: HIGH (prevents resource leaks)**

Defer executes when the surrounding function returns, not at the end of each loop iteration. Placing defer inside a loop means every deferred call accumulates and only runs when the function exits. For resources like file descriptors, network connections, or database handles, this causes a leak that grows with each iteration and can exhaust system limits. Extract the loop body into a separate function so defer runs per iteration.

**Incorrect (what's wrong):**

```go
func readFiles(ch <-chan string) error {
	for path := range ch {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close() // Only closes when readFiles returns — FD leak
		// Process file
	}
	return nil
}
```

**Correct (what's right):**

```go
func readFiles(ch <-chan string) error {
	for path := range ch {
		if err := readFile(path); err != nil {
			return err
		}
	}
	return nil
}

func readFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close() // Closes when readFile returns — per iteration
	// Process file
	return nil
}
```
