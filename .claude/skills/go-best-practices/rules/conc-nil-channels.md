---
title: Nil Channels for Dynamic Select
impact: LOW-MEDIUM
impactDescription: enables dynamic select
tags: channels, nil, select, merge
---

## Nil Channels for Dynamic Select

**Impact: LOW-MEDIUM (enables dynamic select)**

Receiving from or sending to a nil channel blocks forever. Useful to disable a select case by setting the channel to nil after it's closed.

**Incorrect (what's wrong):**

```go
func merge(ch1, ch2 <-chan int) <-chan int {
    ch := make(chan int)
    go func() {
        for { // No way to stop when channels close
            select {
            case v := <-ch1: ch <- v // Receives zero values forever after close
            case v := <-ch2: ch <- v
            }
        }
    }()
    return ch
}
```

**Correct (what's right):**

```go
func merge(ch1, ch2 <-chan int) <-chan int {
    ch := make(chan int, 1)
    go func() {
        for ch1 != nil || ch2 != nil {
            select {
            case v, ok := <-ch1:
                if !ok { ch1 = nil; break } // Disable this case
                ch <- v
            case v, ok := <-ch2:
                if !ok { ch2 = nil; break }
                ch <- v
            }
        }
        close(ch)
    }()
    return ch
}
```
