---
title: Optimize for CPU Cache and Struct Alignment
impact: MEDIUM
impactDescription: improves CPU-bound performance
tags: cpu-cache, cache-line, alignment, struct-layout
---

## Optimize for CPU Cache and Struct Alignment

**Impact: MEDIUM (improves CPU-bound performance)**

CPUs do not fetch individual bytes from memory. They fetch 64-byte cache lines. This has three practical consequences:

**A) Struct field ordering matters.** Go adds padding between fields to satisfy alignment requirements. Ordering fields from largest to smallest minimizes wasted padding.

**B) Slice of structs vs struct of slices.** When iterating over one field of many objects, a slice of structs wastes cache by loading unused fields. A struct of slices (columnar layout) keeps the iterated field contiguous in memory.

**C) Predictable access patterns.** Sequential access (array iteration) triggers CPU prefetching. Random access (pointer chasing, map lookups) defeats it. Prefer sequential iteration when possible.

**Incorrect (what's wrong):**

```go
// A) Poor field ordering — 24 bytes with padding
type Foo struct {
	b   bool  // 1 byte + 7 bytes padding (next field needs 8-byte alignment)
	i64 int64 // 8 bytes
	i32 int32 // 4 bytes + 4 bytes padding (struct must be 8-byte aligned)
}
// sizeof = 24 bytes

// B) Slice of structs — iterating one field loads all fields into cache
type Account struct {
	Name    string  // 16 bytes — loaded but unused
	Balance float64 // 8 bytes — the only field we need
	ID      int64   // 8 bytes — loaded but unused
}

func sumBalances(accounts []Account) float64 {
	var total float64
	for _, a := range accounts {
		total += a.Balance // Each iteration loads 32 bytes, uses 8
	}
	return total
}
```

**Correct (what's right):**

```go
// A) Optimal field ordering — 16 bytes, no wasted padding
type Foo struct {
	i64 int64 // 8 bytes
	i32 int32 // 4 bytes
	b   bool  // 1 byte + 3 bytes padding
}
// sizeof = 16 bytes (8 bytes saved)

// B) Struct of slices — columnar layout for cache-friendly iteration
type Accounts struct {
	Names    []string
	Balances []float64
	IDs      []int64
}

func sumBalances(accounts Accounts) float64 {
	var total float64
	for _, b := range accounts.Balances {
		total += b // Contiguous float64s — perfect cache utilization
	}
	return total
}

// C) Sequential access enables CPU prefetching
// Prefer: iterate over a slice (sequential)
for i := range data {
	process(data[i])
}
// Avoid: chase pointers through a linked list (random access)
for node := head; node != nil; node = node.Next {
	process(node.Value)
}
```
