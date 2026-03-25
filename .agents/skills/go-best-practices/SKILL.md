---
name: go-best-practices
description: Irving's best practices in Go development language, based on "100 Go Mistakes and How to Avoid Them" by Teiva Harsanyi. This skill should be used when writing, reviewing, or refactoring Go code. Triggers on tasks involving Go code organization, data types, concurrency, error handling, testing, or performance optimization.
license: MIT
metadata:
  author: Irving Dinh <irving.dinh@gmail.com>
  version: "1.0.0"
---

# Go Best Practices

Comprehensive guide for Go development, based on "100 Go Mistakes and How to Avoid Them" by Teiva Harsanyi. Contains 88 rules across 10 categories, prioritized by impact to guide automated refactoring and code generation.

## When to Apply

Reference these guidelines when:
- Writing new Go code or packages
- Reviewing code for correctness and performance issues
- Refactoring existing Go code
- Working with concurrency primitives (goroutines, channels, mutexes)
- Implementing error handling patterns
- Optimizing Go application performance
- Writing tests and benchmarks

## Rule Categories by Priority

| Priority | Category | Impact | Prefix |
|----------|----------|--------|--------|
| 1 | Concurrency | CRITICAL | `conc-` |
| 2 | Data Types | HIGH | `data-` |
| 3 | Error Management | HIGH | `error-` |
| 4 | Standard Library | MEDIUM-HIGH | `stdlib-` |
| 5 | Code Organization | MEDIUM | `org-` |
| 6 | Functions & Methods | MEDIUM | `func-` |
| 7 | Control Structures | MEDIUM | `ctrl-` |
| 8 | Strings | MEDIUM | `string-` |
| 9 | Testing | LOW-MEDIUM | `test-` |
| 10 | Optimizations | LOW-MEDIUM | `opt-` |

## Quick Reference

### 1. Code Organization (MEDIUM)

- `org-variable-shadowing` - Avoid shadowed variables to prevent referencing wrong variable
- `org-nested-code` - Keep happy path aligned on the left, reduce nesting
- `org-init-functions` - Prefer ad hoc functions over init for initialization
- `org-getters-setters` - Don't force getters/setters, be pragmatic
- `org-interface-pollution` - Discover abstractions, don't create them prematurely
- `org-interface-producer-side` - Keep interfaces on the consumer side
- `org-returning-interfaces` - Return concrete types, accept interfaces
- `org-any-type` - Only use any when truly needed (e.g., marshaling)
- `org-generics-misuse` - Use generics for concrete needs, not prematurely
- `org-type-embedding` - Don't embed types solely for syntactic sugar
- `org-functional-options` - Use functional options for API-friendly configuration
- `org-project-structure` - Organize by context or layer, stay consistent
- `org-utility-packages` - Name packages by what they provide, not contain
- `org-package-name-collisions` - Avoid variable names that shadow package names
- `org-code-documentation` - Document exported elements with purpose-focused comments
- `org-linters` - Use linters and formatters for quality and consistency

### 2. Data Types (HIGH)

- `data-octal-literals` - Use 0o prefix for octal clarity
- `data-integer-overflows` - Handle silent integer overflows at runtime
- `data-floating-points` - Compare floats within delta, order operations carefully
- `data-slice-length-capacity` - Understand backing array, length, and capacity
- `data-slice-init` - Pre-allocate slices when length is known
- `data-nil-empty-slice` - Understand nil vs empty slice semantics
- `data-checking-slice-empty` - Check length, not nil, for emptiness
- `data-slice-copy` - Copy uses min of two lengths
- `data-slice-append` - Avoid append side effects on shared backing arrays
- `data-slice-memory-leak` - Nil pointer elements excluded by slicing
- `data-map-init` - Pre-allocate maps when size is known
- `data-map-memory-leak` - Maps grow but never shrink
- `data-comparing-values` - Use == for comparable types, reflect.DeepEqual for others

### 3. Control Structures (MEDIUM)

- `ctrl-range-loop-gotchas` - Range copies values and evaluates expression once
- `ctrl-map-iteration` - Map iteration order is non-deterministic
- `ctrl-break-statement` - Break terminates innermost for/switch/select
- `ctrl-defer-loop` - Extract loop body to function for per-iteration defer

### 4. Strings (MEDIUM)

- `string-rune-concept` - A rune is a Unicode code point, 1-4 bytes in UTF-8
- `string-iteration` - Range iterates runes, index accesses bytes
- `string-trim-functions` - TrimRight strips rune set, TrimSuffix strips suffix
- `string-concatenation` - Use strings.Builder for loop concatenation
- `string-conversions` - Use bytes package to avoid extra conversions
- `string-substring-leak` - Substrings share backing array, copy to avoid leaks

### 5. Functions & Methods (MEDIUM)

- `func-receiver-type` - Use pointer for mutation/large structs, value for immutability
- `func-named-results` - Use named results for readability, watch for zero-value traps
- `func-named-result-side-effects` - Named results init to zero, can cause subtle bugs
- `func-nil-receiver` - Don't return nil pointer as interface, return explicit nil
- `func-filename-input` - Accept io.Reader instead of filename
- `func-defer-evaluation` - Defer evaluates arguments immediately, use closures

### 6. Error Management (HIGH)

- `error-panicking` - Reserve panic for programmer errors and missing dependencies
- `error-wrapping` - Wrap with %w for context, %v to hide source error
- `error-comparing-errors` - Use errors.Is and errors.As, not == or type assertions
- `error-handling-discipline` - Handle once, log or return, close with defer

### 7. Concurrency (CRITICAL)

- `conc-concurrency-vs-parallelism` - Concurrency is structure, parallelism is execution
- `conc-not-always-faster` - Benchmark before assuming parallel is faster
- `conc-channels-vs-mutexes` - Mutexes for sync, channels for coordination
- `conc-race-problems` - Data races and race conditions are different bugs
- `conc-workload-types` - CPU-bound: GOMAXPROCS goroutines; I/O-bound: depends
- `conc-context-usage` - Use contexts for deadlines, cancellation, and values
- `conc-context-propagation` - Don't propagate contexts that may cancel prematurely
- `conc-goroutine-lifecycle` - Always have a plan to stop goroutines
- `conc-select-behavior` - Select chooses randomly among ready cases
- `conc-notification-channels` - Use chan struct{} for signals without data
- `conc-nil-channels` - Nil channels block forever, useful to disable select cases
- `conc-channel-size` - Default to size 1 for buffered channels
- `conc-string-formatting` - String formatting can trigger methods and cause deadlocks
- `conc-append-data-race` - Append on shared slice with capacity is a data race
- `conc-mutex-slices-maps` - Slices and maps are pointers, copying doesn't protect data
- `conc-waitgroup` - Call Add before spinning up goroutines
- `conc-sync-cond` - Use sync.Cond for repeated notifications to multiple goroutines
- `conc-errgroup` - Use errgroup for goroutine sync with error handling
- `conc-copying-sync` - Never copy sync types (Mutex, WaitGroup, etc.)

### 8. Standard Library (MEDIUM-HIGH)

- `stdlib-time-duration` - time.Duration is nanoseconds, use time API constants
- `stdlib-json-handling` - Beware embedding, monotonic clock, float64 in map[string]any
- `stdlib-sql-mistakes` - Ping after Open, use prepared statements, handle null
- `stdlib-closing-resources` - Always close io.Closer types to prevent leaks
- `stdlib-http-return` - Return after http.Error to stop handler execution
- `stdlib-default-http` - Configure timeouts for production HTTP clients and servers

### 9. Testing (LOW-MEDIUM)

- `test-execution-modes` - Use -race, -parallel, -shuffle, and build tags
- `test-table-driven` - Group similar tests in table-driven format
- `test-sleeping` - Use synchronization, not sleep, for reliable tests
- `test-time-api` - Inject time dependency for testable time-dependent code
- `test-utility-packages` - Use httptest and iotest for HTTP and I/O testing
- `test-benchmarks` - Reset timer, prevent compiler optimization, avoid observer effect
- `test-features` - Use coverage, test from different packages, utility functions
- `test-fuzzing` - Use fuzzing to discover unexpected inputs and bugs

### 10. Optimizations (LOW-MEDIUM)

- `opt-cpu-cache-alignment` - Optimize for cache lines, spatial locality, data alignment
- `opt-false-sharing` - Pad struct fields to prevent concurrent cache line invalidation
- `opt-heap-allocations` - Prefer stack allocations, use sync.Pool, reduce escapes
- `opt-inlining` - Use fast-path inlining to reduce call overhead
- `opt-diagnostics` - Use pprof for CPU/heap/goroutine profiling and execution tracer
- `opt-gc` - Tune GOGC and GOMEMLIMIT for GC performance

## How to Use

Read individual rule files for detailed explanations and code examples:

```
rules/conc-race-problems.md
rules/data-slice-length-capacity.md
```

Each rule file contains:
- Brief explanation of why it matters
- Incorrect code example with explanation
- Correct code example with explanation
- Additional context and references

## Full Compiled Document

For the complete guide with all rules expanded: `AGENTS.md`
