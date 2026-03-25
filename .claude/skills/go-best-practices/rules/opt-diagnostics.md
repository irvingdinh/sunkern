---
title: Profile Before Optimizing
impact: HIGH
impactDescription: enables data-driven optimization
tags: profiling, pprof, tracing, diagnostics
---

## Profile Before Optimizing

**Impact: HIGH (enables data-driven optimization)**

Never optimize based on guesses. Go provides powerful profiling and tracing tools built into the standard library. Use them to identify actual bottlenecks before making changes.

- **CPU profile**: Shows where CPU time is spent.
- **Heap profile**: Shows current allocations and where they come from.
- **Goroutine profile**: Shows all goroutine stack traces (useful for leak detection).
- **Mutex profile**: Shows where goroutines block on mutexes.
- **Block profile**: Shows where goroutines block on channel/sync operations.
- **Execution tracer**: Shows goroutine scheduling, GC events, and syscalls on a timeline.

Enabling pprof in production is safe -- it has negligible overhead until actively profiled.

**Incorrect (what's wrong):**

```go
// Guessing where performance issues are without profiling
func slowHandler(w http.ResponseWriter, r *http.Request) {
	// "This loop looks slow, let me optimize it"
	// (But the real bottleneck is the database query below)
	data := processItems(items) // Optimized this — no improvement

	result := db.Query(query) // Actual bottleneck — never profiled
	json.NewEncoder(w).Encode(result)
}
```

**Correct (what's right):**

```go
// Enable pprof endpoint in your application
import _ "net/http/pprof"

func main() {
	// pprof endpoints are automatically registered on DefaultServeMux
	// For custom mux, register manually:
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)

	go http.ListenAndServe(":6060", nil) // Separate debug server
	// ...
}

// Collect and analyze profiles:
//
// CPU profile (30 seconds):
//   curl -o cpu.out http://localhost:6060/debug/pprof/profile?seconds=30
//   go tool pprof -http=:8080 cpu.out
//
// Heap profile (trigger GC first for accuracy):
//   curl -o heap.out http://localhost:6060/debug/pprof/heap?gc=1
//   go tool pprof -http=:8080 heap.out
//
// Goroutine profile (find leaks):
//   curl -o goroutine.out http://localhost:6060/debug/pprof/goroutine?debug=0
//   go tool pprof -http=:8080 goroutine.out
//
// From benchmarks:
//   go test -bench=. -cpuprofile=cpu.out -memprofile=mem.out
//   go tool pprof -http=:8080 cpu.out
//
// Execution tracer (goroutine scheduling, GC timeline):
//   curl -o trace.out http://localhost:6060/debug/pprof/trace?seconds=5
//   go tool trace trace.out
```
