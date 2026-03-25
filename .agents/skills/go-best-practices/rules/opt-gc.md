---
title: Tune GC with GOGC and GOMEMLIMIT
impact: MEDIUM
impactDescription: handles load spikes
tags: gc, gogc, gomemlimit, memory
---

## Tune GC with GOGC and GOMEMLIMIT

**Impact: MEDIUM (handles load spikes)**

Go's garbage collector has two tuning knobs:

**GOGC** (default 100): Controls GC frequency as a percentage of heap growth. `GOGC=100` means the GC triggers when the heap has doubled since the last collection. Lower values mean more frequent GC (less memory, more CPU). Higher values mean less frequent GC (more memory, less CPU). `GOGC=off` disables the GOGC trigger entirely.

**GOMEMLIMIT** (Go 1.19+): Sets a soft memory limit. When the heap approaches this limit, the GC works harder to stay under it. This prevents OOM kills under sudden load spikes and is the recommended way to control memory usage in containerized environments.

The combination `GOGC=off GOMEMLIMIT=512MiB` tells the GC to only collect when approaching the memory limit, maximizing throughput when memory is available.

**Incorrect (what's wrong):**

```go
// Default GOGC=100 with no memory limit
// Under sudden load:
// 1. Heap grows rapidly
// 2. GC triggers at 2x, but allocations outpace collection
// 3. Heap keeps growing until the container is OOM-killed
// 4. No protection against memory exhaustion

// Lowering GOGC without GOMEMLIMIT wastes CPU during normal load
// GOGC=50 — GC runs twice as often even when memory is plentiful
```

**Correct (what's right):**

```go
// For memory-constrained environments (e.g., 512MiB container):
// Set via environment variables:
//   GOMEMLIMIT=512MiB  — soft limit, GC works harder to stay under
//   GOGC=100           — normal GC behavior with memory safety net

// For maximum throughput with bounded memory:
//   GOGC=off GOMEMLIMIT=512MiB — GC only when approaching limit

// Set programmatically:
import "runtime/debug"

func init() {
	// More frequent GC — trades CPU for lower memory usage
	debug.SetGCPercent(50)

	// Soft memory limit — GC intensifies near the limit
	debug.SetMemoryLimit(512 << 20) // 512 MiB
}

// Monitor GC behavior with runtime metrics:
import "runtime"

func printGCStats() {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	fmt.Printf("HeapAlloc: %d MiB\n", stats.HeapAlloc/1024/1024)
	fmt.Printf("NumGC: %d\n", stats.NumGC)
	fmt.Printf("GCCPUFraction: %.4f\n", stats.GCCPUFraction)
}

// Guidelines:
// - Container with 1GiB limit → GOMEMLIMIT=900MiB (leave headroom for goroutine stacks, non-heap memory)
// - CPU-sensitive service → higher GOGC (200-500) with GOMEMLIMIT as safety net
// - Memory-sensitive service → lower GOGC (50) or GOGC=off with tight GOMEMLIMIT
```
