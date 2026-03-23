---
title: sync.Cond for Repeated Broadcast
impact: LOW
impactDescription: enables repeated broadcast
tags: sync, cond, broadcast, notification
---

## sync.Cond for Repeated Broadcast

**Impact: LOW (enables repeated broadcast)**

sync.Cond allows broadcasting a signal to ALL waiting goroutines repeatedly. Channels can only be closed once.

**Incorrect (what's wrong):**

```go
// Using channel for repeated notifications
ch := make(chan struct{})
close(ch) // Can only close once, panics on second close
```

**Correct (what's right):**

```go
cond := sync.NewCond(&sync.Mutex{})
// Waiter:
cond.L.Lock()
for !condition() { cond.Wait() }
cond.L.Unlock()
// Notifier:
cond.L.Lock()
// update state
cond.Broadcast() // Wakes ALL waiters, can be called repeatedly
cond.L.Unlock()
```
