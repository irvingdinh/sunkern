---
title: Use Explicit time.Duration Units
impact: MEDIUM
impactDescription: prevents nanosecond confusion
tags: time, duration, nanoseconds
---

## Use Explicit time.Duration Units

**Impact: MEDIUM (prevents nanosecond confusion)**

`time.Duration` is an `int64` representing nanoseconds. Passing a raw integer literal creates a duration in nanoseconds, not milliseconds or seconds. This is a common mistake that leads to timers and tickers firing far more frequently than intended (or appearing to do nothing because the interval is microscopically small).

Always multiply by a `time.Duration` constant (`time.Second`, `time.Millisecond`, etc.) to express intent clearly.

**Incorrect (what's wrong):**

```go
// Intended: tick every 1 second
// Actual: tick every 1000 nanoseconds = 1 microsecond
ticker := time.NewTicker(1000)

// Intended: 5-second timeout
// Actual: 5-nanosecond timeout
ctx, cancel := context.WithTimeout(ctx, 5)
defer cancel()
```

**Correct (what's right):**

```go
// Explicit: tick every 1 second
ticker := time.NewTicker(time.Second)

// Explicit: tick every 1000 milliseconds (also 1 second)
ticker := time.NewTicker(1000 * time.Millisecond)

// Explicit: 5-second timeout
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
```
