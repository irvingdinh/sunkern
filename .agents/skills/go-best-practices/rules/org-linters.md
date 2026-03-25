---
title: Use Linters and Formatters
impact: MEDIUM
impactDescription: Without automated linting, style inconsistencies and detectable bugs slip through
tags: linters, formatting, quality
---

## Use Linters and Formatters

**Impact: MEDIUM (Without automated linting, style inconsistencies and detectable bugs slip through)**

Use linters (`go vet`, `errcheck`, `golangci-lint`) and formatters (`gofmt`, `goimports`) to catch errors and enforce consistency. Automate them in CI so issues are caught before code review. `go vet` detects suspicious constructs, `errcheck` finds unchecked errors, and `golangci-lint` aggregates dozens of linters into a single tool.

**Incorrect (what's wrong):**

```go
// No linting — these issues go undetected:

func process(ch chan int) {
    // unreachable code after return — caught by go vet
    return
    fmt.Println("done")
}

func readFile(path string) {
    f, _ := os.Open(path) // unchecked error — caught by errcheck
    defer f.Close()
}

func compare(a, b []byte) bool {
    return a == nil && b == nil // suspicious comparison — caught by staticcheck
}
```

**Correct (what's right):**

```go
// All issues caught and fixed:

func process(ch chan int) {
    fmt.Println("done")
}

func readFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("opening file: %w", err)
    }
    defer f.Close()
    // ... use f
    return nil
}

func compare(a, b []byte) bool {
    return bytes.Equal(a, b)
}
```

Example `.golangci.yml` configuration:

```go
// Run golangci-lint in your project:
//   golangci-lint run ./...
//
// Run go vet:
//   go vet ./...
//
// Format code:
//   gofmt -w .
//   goimports -w .
//
// Example .golangci.yml:
//
// linters:
//   enable:
//     - errcheck
//     - govet
//     - staticcheck
//     - unused
//     - gosimple
//     - ineffassign
//     - typecheck
//     - misspell
//     - revive
//     - gocritic
//     - shadow
//
// linters-settings:
//   govet:
//     enable-all: true
//   revive:
//     rules:
//       - name: unexported-return
//       - name: unused-parameter
//
// run:
//   timeout: 5m
```

Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #16.
