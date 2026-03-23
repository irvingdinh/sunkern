---
title: Channel Size Selection
impact: MEDIUM
impactDescription: correct buffering choice
tags: channels, buffered, unbuffered, size
---

## Channel Size Selection

**Impact: MEDIUM (correct buffering choice)**

Unbuffered channels provide synchronization guarantees. For buffered channels, default to size 1 unless you have a specific reason (worker pool, rate limiting).

**Incorrect (what's wrong):**

```go
ch := make(chan Result, 1000) // Arbitrary large buffer hides backpressure
```

**Correct (what's right):**

```go
ch := make(chan Result)    // Unbuffered: strong synchronization
ch := make(chan Result, 1) // Buffered: default minimum when needed
ch := make(chan Result, numWorkers) // Sized to worker pool
```
