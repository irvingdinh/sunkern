---
title: Concurrency Is Not Always Faster
impact: HIGH
impactDescription: prevents wasted parallelization
tags: concurrency, performance, benchmarks
---

## Concurrency Is Not Always Faster

**Impact: HIGH (prevents wasted parallelization)**

If workload per goroutine is too small, goroutine creation and scheduling overhead destroys the benefit. Always benchmark sequential vs concurrent.

**Incorrect (what's wrong):**

```go
func parallelMergesort(s []int) {
    if len(s) <= 1 { return }
    middle := len(s) / 2
    var wg sync.WaitGroup
    wg.Add(2)
    go func() { defer wg.Done(); parallelMergesort(s[:middle]) }()
    go func() { defer wg.Done(); parallelMergesort(s[middle:]) }()
    wg.Wait()
    merge(s, middle) // Spins goroutines for tiny slices — 10x slower
}
```

**Correct (what's right):**

```go
const threshold = 2048
func parallelMergesort(s []int) {
    if len(s) <= 1 { return }
    if len(s) <= threshold {
        sequentialMergesort(s); return // Small slices: sequential
    }
    middle := len(s) / 2
    var wg sync.WaitGroup
    wg.Add(2)
    go func() { defer wg.Done(); parallelMergesort(s[:middle]) }()
    go func() { defer wg.Done(); parallelMergesort(s[middle:]) }()
    wg.Wait()
    merge(s, middle)
}
```
