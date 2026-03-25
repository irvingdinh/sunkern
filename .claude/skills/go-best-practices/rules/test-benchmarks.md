---
title: Write Accurate Benchmarks
impact: HIGH
impactDescription: prevents inaccurate measurements
tags: benchmarks, performance, compiler, observer-effect
---

## Write Accurate Benchmarks

**Impact: HIGH (prevents inaccurate measurements)**

Go benchmarks have four common traps that produce misleading results:

**A) Reset/pause timer for setup.** If benchmark setup is expensive, it inflates the measured time. Use `b.ResetTimer()` after setup or `b.StopTimer()`/`b.StartTimer()` around setup within the loop.

**B) Micro-benchmarks need statistical rigor.** A single run is noisy. Use `-count=N` and `benchstat` to get statistically meaningful comparisons.

**C) Assign results to prevent compiler elimination.** If the result of a function is unused, the compiler may optimize the call away entirely, benchmarking nothing.

**D) Re-create data each iteration to avoid CPU cache observer effect.** If you reuse the same small input, it stays in L1 cache and you measure cache-hot performance, not realistic throughput.

**Incorrect (what's wrong):**

```go
// A) Setup time included in benchmark
func BenchmarkProcess(b *testing.B) {
	data := expensiveSetup() // This time is measured too
	for i := 0; i < b.N; i++ {
		process(data)
	}
}

// C) Compiler may eliminate the call entirely
func BenchmarkPopcnt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		popcnt(uint64(i)) // Result unused — compiler can remove this
	}
}

// D) Same input every iteration — always in cache
func BenchmarkSort(b *testing.B) {
	data := []int{5, 3, 1, 4, 2}
	for i := 0; i < b.N; i++ {
		sort.Ints(data) // After first iteration, data is sorted — measures nothing
	}
}
```

**Correct (what's right):**

```go
// A) Reset timer after setup
func BenchmarkProcess(b *testing.B) {
	data := expensiveSetup()
	b.ResetTimer() // Excludes setup from measurement
	for i := 0; i < b.N; i++ {
		process(data)
	}
}

// C) Assign result to package-level var to prevent elimination
var global uint64

func BenchmarkPopcnt(b *testing.B) {
	var v uint64
	for i := 0; i < b.N; i++ {
		v = popcnt(uint64(i))
	}
	global = v // Prevents compiler optimization
}

// D) Re-create input each iteration
func BenchmarkSort(b *testing.B) {
	original := generateRandomSlice(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data := make([]int, len(original))
		copy(data, original) // Fresh unsorted copy each iteration
		sort.Ints(data)
	}
}

// B) Run with -count and use benchstat for comparison:
// go test -bench=BenchmarkSort -count=10 > old.txt
// # make changes
// go test -bench=BenchmarkSort -count=10 > new.txt
// benchstat old.txt new.txt
```
