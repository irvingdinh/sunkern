---
title: Avoid Memory Leaks from Slice References
impact: HIGH
impactDescription: prevents memory leaks
tags: slices, memory-leak, pointers, gc
---

## Avoid Memory Leaks from Slice References

**Impact: HIGH (prevents memory leaks)**

Slicing a large slice (e.g., `large[:3]`) returns a new slice header that still points to the original backing array. As long as the sub-slice is reachable, the garbage collector cannot reclaim the entire backing array, even if you only need a few elements. This can cause significant memory waste when extracting small portions of large slices.

For slices of pointer types or types containing pointers, there is an additional concern: pointers in the backing array beyond the sub-slice's length remain reachable and prevent the objects they point to from being garbage collected. Explicitly nil out excluded pointer elements if not copying.

**Incorrect (what's wrong):**

```go
func getFirst(large [][]byte) [][]byte {
	return large[:3] // Keeps entire backing array alive
}
```

**Correct (what's right):**

```go
func getFirst(large [][]byte) [][]byte {
	result := make([][]byte, 3)
	copy(result, large[:3])
	return result // Original backing array can be GC'd
}

// For pointer slices without full copy, nil out excluded elements:
func getFirstWithCleanup(large []*BigStruct) []*BigStruct {
	result := large[:3:3]
	for i := 3; i < len(large); i++ {
		large[i] = nil // Allow GC to reclaim these objects
	}
	return result
}
```
