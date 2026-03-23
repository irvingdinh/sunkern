---
title: Understand Runes and UTF-8 Encoding
impact: HIGH
impactDescription: foundational string knowledge
tags: runes, unicode, utf-8, encoding
---

## Understand Runes and UTF-8 Encoding

**Impact: HIGH (foundational string knowledge)**

A rune is a Unicode code point, defined as an alias for int32. Go strings are sequences of bytes encoded in UTF-8, where a single code point may occupy 1 to 4 bytes. The built-in len() function returns the byte count of a string, not the number of runes. This distinction is critical when working with non-ASCII text, as byte length and character count diverge.

**Incorrect (what's wrong):**

```go
s := "hêllo"
fmt.Println(len(s)) // 6 (bytes), not 5 (runes)
```

**Correct (what's right):**

```go
s := "hêllo"
fmt.Println(utf8.RuneCountInString(s)) // 5 (runes)
fmt.Println(len([]rune(s)))            // 5 (runes)
```
