---
title: Document Exported Elements Properly
impact: LOW
impactDescription: Missing or poorly written documentation makes packages harder to use
tags: documentation, comments, godoc
---

## Document Exported Elements Properly

**Impact: LOW (Missing or poorly written documentation makes packages harder to use)**

Every exported element (function, type, constant, variable, method) must be documented. Start the comment with the element's name. Focus on what the element does, not how it does it. Godoc uses these comments to generate documentation, and they appear in IDE tooltips. A missing or vague comment forces readers to read the implementation.

**Incorrect (what's wrong):**

```go
// This function uses a loop to iterate
func Add(a, b int) int { return a + b }

// helper
func ParseConfig(path string) (*Config, error) { /* ... */ return nil, nil }

type Worker struct { // no comment
    ID int
}

// does stuff
func (w *Worker) Run() error { return nil }
```

**Correct (what's right):**

```go
// Add returns the sum of a and b.
func Add(a, b int) int { return a + b }

// ParseConfig reads the configuration file at path and returns
// the parsed Config. It returns an error if the file cannot be
// read or contains invalid YAML.
func ParseConfig(path string) (*Config, error) { /* ... */ return nil, nil }

// Worker represents a background task processor.
type Worker struct {
    // ID uniquely identifies this worker within a pool.
    ID int
}

// Run starts the worker's processing loop. It blocks until the
// worker finishes or encounters an unrecoverable error.
func (w *Worker) Run() error { return nil }
```

When documenting a package, add a comment to the `doc.go` file or above the `package` declaration. Use `// Deprecated:` to mark deprecated elements. Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #15.
