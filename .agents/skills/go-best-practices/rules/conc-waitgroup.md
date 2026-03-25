---
title: WaitGroup Add Before Goroutine
impact: HIGH
impactDescription: prevents non-deterministic results
tags: waitgroup, goroutines, synchronization
---

## WaitGroup Add Before Goroutine

**Impact: HIGH (prevents non-deterministic results)**

Call wg.Add() BEFORE starting the goroutine, not inside it. Otherwise Wait() may return before Add() is called.

**Incorrect (what's wrong):**

```go
wg := sync.WaitGroup{}
var v uint64
for i := 0; i < 3; i++ {
    go func() {
        wg.Add(1) // Race: Wait() may return before this runs
        atomic.AddUint64(&v, 1)
        wg.Done()
    }()
}
wg.Wait() // May print 0, 1, 2, or 3
```

**Correct (what's right):**

```go
wg := sync.WaitGroup{}
var v uint64
for i := 0; i < 3; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        atomic.AddUint64(&v, 1)
    }()
}
wg.Wait() // Always prints 3
```
