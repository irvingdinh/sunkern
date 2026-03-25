---
title: CPU-Bound vs I/O-Bound Workloads
impact: MEDIUM
impactDescription: right goroutine count
tags: goroutines, cpu-bound, io-bound, gomaxprocs
---

## CPU-Bound vs I/O-Bound Workloads

**Impact: MEDIUM (right goroutine count)**

CPU-bound: limit goroutines to runtime.GOMAXPROCS. I/O-bound: goroutine count depends on external system capacity.

**Incorrect (what's wrong):**

```go
// CPU-bound work with 10000 goroutines on 4-core machine
for i := 0; i < 10000; i++ {
    go cpuIntensiveWork(i)
}
```

**Correct (what's right):**

```go
numWorkers := runtime.GOMAXPROCS(0) // e.g., 4
ch := make(chan int, 10000)
for i := 0; i < numWorkers; i++ {
    go func() { for n := range ch { cpuIntensiveWork(n) } }()
}
```
