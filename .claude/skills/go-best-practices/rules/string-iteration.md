---
title: Iterate Strings by Rune, Not by Byte
impact: HIGH
impactDescription: prevents byte/rune confusion
tags: strings, iteration, runes, range
---

## Iterate Strings by Rune, Not by Byte

**Impact: HIGH (prevents byte/rune confusion)**

Range over a string iterates by rune, yielding the byte-position index and the decoded rune at each step. However, indexing a string with s[i] accesses a single byte, not a rune. Mixing these two behaviors produces garbled output for any string containing multi-byte characters. Always use the rune variable from the range clause. To access the nth rune by position, convert the string to a []rune slice first.

**Incorrect (what's wrong):**

```go
s := "hêllo"
for i := range s {
	fmt.Printf("position %d: %c\n", i, s[i]) // s[i] is a byte, not a rune
}
// Prints: h, Ã (broken), l, l, o
```

**Correct (what's right):**

```go
s := "hêllo"
for i, r := range s {
	fmt.Printf("position %d: %c\n", i, r) // r is the rune
}
// Prints: h, ê, l, l, o
```

To access the ith rune, convert to []rune first:

```go
r := []rune(s)[1] // 'ê'
```
