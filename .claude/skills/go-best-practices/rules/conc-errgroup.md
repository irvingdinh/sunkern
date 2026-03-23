---
title: errgroup for Goroutine Error Handling
impact: MEDIUM
impactDescription: simplifies goroutine error handling
tags: errgroup, goroutines, errors, context
---

## errgroup for Goroutine Error Handling

**Impact: MEDIUM (simplifies goroutine error handling)**

errgroup synchronizes goroutines, collects the first error, and optionally cancels remaining work via context.

**Incorrect (what's wrong):**

```go
var wg sync.WaitGroup
var mu sync.Mutex
var firstErr error
for _, url := range urls {
    wg.Add(1)
    go func(u string) {
        defer wg.Done()
        if err := fetch(u); err != nil {
            mu.Lock()
            if firstErr == nil { firstErr = err }
            mu.Unlock()
        }
    }(url)
}
wg.Wait()
```

**Correct (what's right):**

```go
g, ctx := errgroup.WithContext(context.Background())
for _, url := range urls {
    g.Go(func() error {
        return fetch(ctx, url)
    })
}
if err := g.Wait(); err != nil { return err }
```
