---
title: Use TrimSuffix/TrimPrefix Instead of TrimRight/TrimLeft
impact: LOW-MEDIUM
impactDescription: prevents incorrect trimming
tags: strings, trim, suffix, prefix
---

## Use TrimSuffix/TrimPrefix Instead of TrimRight/TrimLeft

**Impact: LOW-MEDIUM (prevents incorrect trimming)**

TrimRight and TrimLeft strip all trailing or leading runes that appear anywhere in a given set of characters. TrimSuffix and TrimPrefix remove an exact substring. Confusing the two leads to over-trimming, where individual characters from the intended suffix or prefix are stripped independently rather than matched as a whole string.

**Incorrect (what's wrong):**

```go
fmt.Println(strings.TrimRight("123oxo", "xo")) // "123" — removes all trailing x and o runes
// Developer expected "123o" (remove suffix "xo")
```

**Correct (what's right):**

```go
fmt.Println(strings.TrimSuffix("123oxo", "xo")) // "123o" — removes exact suffix
```
