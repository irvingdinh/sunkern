---
title: Don't Create Utility Packages
impact: LOW-MEDIUM
impactDescription: Packages named util, common, or shared are grab-bags that grow unbounded
tags: packages, naming, organization
---

## Don't Create Utility Packages

**Impact: LOW-MEDIUM (Packages named util, common, or shared are grab-bags that grow unbounded)**

Don't create packages named `common`, `util`, `shared`, `helpers`, or `base`. These names describe nothing about what the package provides. They become dumping grounds for unrelated code and grow without bound. Name packages after what they provide so the call site reads naturally.

**Incorrect (what's wrong):**

```go
package util

func NewStringSet(values ...string) map[string]struct{} {
    m := make(map[string]struct{}, len(values))
    for _, v := range values {
        m[v] = struct{}{}
    }
    return m
}

func Contains(set map[string]struct{}, key string) bool {
    _, ok := set[key]
    return ok
}

// usage: util.NewStringSet("a", "b")
// usage: util.Contains(set, "a")
```

**Correct (what's right):**

```go
package stringset

type Set map[string]struct{}

func New(values ...string) Set {
    m := make(Set, len(values))
    for _, v := range values {
        m[v] = struct{}{}
    }
    return m
}

func (s Set) Contains(key string) bool {
    _, ok := s[key]
    return ok
}

// usage: stringset.New("a", "b")
// usage: s.Contains("a")
```

The call site `stringset.New(...)` is self-documenting. If a utility function doesn't fit in any domain package, that's a sign you need a new small, focused package — not a bigger `util`. Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #13.
