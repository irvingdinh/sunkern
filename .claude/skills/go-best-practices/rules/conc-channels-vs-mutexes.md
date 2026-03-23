---
title: Channels vs Mutexes
impact: HIGH
impactDescription: pick the right tool
tags: channels, mutexes, synchronization, coordination
---

## Channels vs Mutexes

**Impact: HIGH (pick the right tool)**

Parallel goroutines sharing state → mutexes. Concurrent goroutines coordinating/transferring ownership → channels.

**Incorrect (what's wrong):**

```go
// Using channel as mutex
ch := make(chan struct{}, 1)
ch <- struct{}{}
// critical section
<-ch
```

**Correct (what's right):**

```go
// Shared state → mutex
var mu sync.Mutex
mu.Lock()
counter++
mu.Unlock()

// Coordination/ownership transfer → channel
results := make(chan Result)
go func() { results <- doWork() }()
r := <-results
```
