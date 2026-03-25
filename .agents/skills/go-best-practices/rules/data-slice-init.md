---
title: Pre-allocate Slices When Length Is Known
impact: MEDIUM
impactDescription: reduces allocations
tags: slices, initialization, performance
---

## Pre-allocate Slices When Length Is Known

**Impact: MEDIUM (reduces allocations)**

When the final length of a slice is known or can be estimated, initialize it with the appropriate length or capacity using `make`. A nil or zero-capacity slice that grows via repeated `append` calls triggers multiple backing array reallocations and copies, which wastes CPU and memory. Pre-allocating avoids this overhead entirely.

Use `make([]T, 0, n)` with `append` when you want bounds safety, or `make([]T, n)` with direct index assignment when filling every position.

**Incorrect (what's wrong):**

```go
var ids []string
for _, user := range users {
	ids = append(ids, user.ID) // Multiple reallocations as slice grows
}
```

**Correct (what's right):**

```go
ids := make([]string, 0, len(users))
for _, user := range users {
	ids = append(ids, user.ID)
}

// Or: allocate with length and assign by index
ids := make([]string, len(users))
for i, user := range users {
	ids[i] = user.ID
}
```
