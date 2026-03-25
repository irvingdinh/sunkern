---
title: Avoid Unnecessary String/Byte Conversions
impact: LOW-MEDIUM
impactDescription: avoids unnecessary allocations
tags: strings, bytes, conversion, io
---

## Avoid Unnecessary String/Byte Conversions

**Impact: LOW-MEDIUM (avoids unnecessary allocations)**

The bytes package mirrors most functions in the strings package. When I/O operations return []byte, converting to string just to use the strings package wastes an allocation and a copy. Work with the bytes package directly to process the data in its original form.

**Incorrect (what's wrong):**

```go
func process(data []byte) {
	s := string(data)                      // Unnecessary conversion
	if strings.Contains(s, "error") { /* ... */ }
}
```

**Correct (what's right):**

```go
func process(data []byte) {
	if bytes.Contains(data, []byte("error")) { /* ... */ } // No conversion
}
```
