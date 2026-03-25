---
title: Accept io.Reader Instead of Filenames
impact: MEDIUM
impactDescription: improves reusability and testability
tags: io, reader, testing, abstraction
---

## Accept io.Reader Instead of Filenames

**Impact: MEDIUM (improves reusability and testability)**

Accept io.Reader instead of a filename string. This makes functions work with files, strings, HTTP bodies, pipes, and test data without modification. The caller decides the source, and the function focuses on processing. This follows the dependency inversion principle and makes unit testing straightforward since no temporary files are needed.

**Incorrect (what's wrong):**

```go
func countLines(filename string) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}
```

**Correct (what's right):**

```go
func countLines(r io.Reader) (int, error) {
	scanner := bufio.NewScanner(r)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}
// Usage: countLines(os.Stdin), countLines(strings.NewReader("test\ndata"))
```
