---
title: Select Random Behavior
impact: MEDIUM
impactDescription: prevents message loss
tags: select, channels, random, determinism
---

## Select Random Behavior

**Impact: MEDIUM (prevents message loss)**

When multiple select cases are ready, Go picks ONE randomly (not source order). This can cause unexpected message loss.

**Incorrect (what's wrong):**

```go
for {
    select {
    case v := <-messageCh:  fmt.Println(v)
    case <-disconnectCh:    fmt.Println("disconnected"); return
    }
}
// May disconnect before consuming all messages
```

**Correct (what's right):**

```go
for {
    select {
    case v, ok := <-messageCh:
        if !ok { return } // Channel closed = disconnect
        fmt.Println(v)
    }
}
```
