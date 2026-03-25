---
title: Use Explicit Octal Literal Prefix
impact: LOW
impactDescription: prevents misread literals
tags: integers, octal, readability
---

## Use Explicit Octal Literal Prefix

**Impact: LOW (prevents misread literals)**

In Go, integer literals starting with `0` are interpreted as octal (base 8). This is a common source of confusion, especially for developers coming from languages where leading zeros have no special meaning. Go 1.13 introduced the `0o` prefix to make octal literals explicit and unambiguous. Always prefer `0o` for octal values.

Go also supports `0b` for binary literals, `0x` for hexadecimal literals, and `_` as a digit separator for readability (e.g., `1_000_000`).

**Incorrect (what's wrong):**

```go
perms := 0644 // Octal, but looks like decimal to newcomers
```

**Correct (what's right):**

```go
perms := 0o644 // Explicitly octal

// Other useful literal prefixes and separators:
binary := 0b1010_0001   // Binary with separator
hex := 0xFF             // Hexadecimal
million := 1_000_000    // Decimal with separator for readability
```
