---
title: Concurrency vs Parallelism
impact: MEDIUM
impactDescription: foundational understanding
tags: concurrency, parallelism, concepts
---

## Concurrency vs Parallelism

**Impact: MEDIUM (foundational understanding)**

Concurrency is about STRUCTURE (dealing with multiple things), parallelism is about EXECUTION (doing multiple things simultaneously). Concurrency enables parallelism.

**Incorrect (what's wrong):**

```go
// Assuming more goroutines = faster
func processAll(items []Item) {
    for _, item := range items {
        go process(item) // Thousands of goroutines for CPU-bound work
    }
}
```

**Correct (what's right):**

```go
func processAll(items []Item) {
    numWorkers := runtime.GOMAXPROCS(0)
    ch := make(chan Item, len(items))
    for _, item := range items { ch <- item }
    close(ch)
    var wg sync.WaitGroup
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for item := range ch { process(item) }
        }()
    }
    wg.Wait()
}
```
