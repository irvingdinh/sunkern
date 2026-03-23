---
title: Be Careful with Type Embedding
impact: MEDIUM
impactDescription: Embedding promotes all fields and methods, potentially exposing internals
tags: embedding, composition, visibility
---

## Be Careful with Type Embedding

**Impact: MEDIUM (Embedding promotes all fields and methods, potentially exposing internals)**

Type embedding promotes all of the embedded type's exported fields and methods to the outer type. Don't use it solely for syntactic convenience. The main risk is unintentionally exposing methods that should remain internal. Embedding `sync.Mutex` in a struct, for example, lets any caller lock and unlock it directly — breaking encapsulation.

**Incorrect (what's wrong):**

```go
type InMem struct {
    sync.Mutex
    m map[string]int
}

func New() *InMem {
    return &InMem{m: make(map[string]int)}
}

// Client can call inMem.Lock() and inMem.Unlock() directly — dangerous
```

**Correct (what's right):**

```go
type InMem struct {
    mu sync.Mutex
    m  map[string]int
}

func New() *InMem {
    return &InMem{m: make(map[string]int)}
}

func (s *InMem) Get(key string) (int, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    v, ok := s.m[key]
    return v, ok
}
```

Use embedding when you genuinely want to promote behavior — for example, embedding `io.Reader` in a struct that wraps a reader. Don't use it when you just want to save a few keystrokes on method calls. Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #10.
