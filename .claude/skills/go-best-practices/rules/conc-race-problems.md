---
title: Data Races vs Race Conditions
impact: CRITICAL
impactDescription: causes undefined behavior
tags: data-race, race-condition, atomics, mutex
---

## Data Races vs Race Conditions

**Impact: CRITICAL (causes undefined behavior)**

A data race: multiple goroutines access same memory, at least one writes. A race condition: behavior depends on uncontrolled timing. Data-race-free != deterministic.

**Incorrect (what's wrong):**

```go
var count int64
go func() { count++ }() // Data race — concurrent write
go func() { count++ }()
```

**Correct (what's right):**

```go
var count int64
go func() { atomic.AddInt64(&count, 1) }()
go func() { atomic.AddInt64(&count, 1) }()
```
