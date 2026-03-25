---
title: Prevent False Sharing with Cache Line Padding
impact: HIGH
impactDescription: prevents concurrent cache invalidation
tags: false-sharing, cache-line, padding, concurrency
---

## Prevent False Sharing with Cache Line Padding

**Impact: HIGH (prevents concurrent cache invalidation)**

When two goroutines write to different variables that happen to reside in the same 64-byte CPU cache line, each write invalidates the entire cache line on the other core. This is called "false sharing" -- the variables are logically independent, but physically share a cache line. The result is constant cache-line bouncing between cores, which can degrade performance by an order of magnitude.

The fix is to add padding between independently written fields so they occupy different cache lines.

**Incorrect (what's wrong):**

```go
type Result struct {
	sumA int64 // 8 bytes
	sumB int64 // 8 bytes — same cache line as sumA
}

func compute(r *Result) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1_000_000; i++ {
			atomic.AddInt64(&r.sumA, 1) // Invalidates sumB's cache line
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 1_000_000; i++ {
			atomic.AddInt64(&r.sumB, 1) // Invalidates sumA's cache line
		}
	}()

	wg.Wait()
}
```

**Correct (what's right):**

```go
type Result struct {
	sumA int64
	_    [56]byte // Padding: 64 (cache line) - 8 (int64) = 56 bytes
	sumB int64    // Now in a different cache line
}

func compute(r *Result) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1_000_000; i++ {
			atomic.AddInt64(&r.sumA, 1) // No false sharing — independent cache lines
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 1_000_000; i++ {
			atomic.AddInt64(&r.sumB, 1) // No false sharing — independent cache lines
		}
	}()

	wg.Wait()
}

// Common pattern: per-CPU counters with padding
type PaddedCounter struct {
	value int64
	_     [56]byte
}

type ShardedCounter struct {
	counters []PaddedCounter // Each counter on its own cache line
}
```
