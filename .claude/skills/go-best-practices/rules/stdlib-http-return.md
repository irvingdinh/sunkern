---
title: Return After http.Error
impact: HIGH
impactDescription: prevents double response writes
tags: http, handler, return, error
---

## Return After http.Error

**Impact: HIGH (prevents double response writes)**

`http.Error` writes an error response to the client, but it does NOT stop handler execution. Without an explicit `return`, the handler continues and may write a second response, causing a "superfluous response.WriteHeader call" warning and sending corrupted output to the client.

This applies to any response-writing function: `http.Error`, `http.Redirect`, `json.NewEncoder(w).Encode()`, `w.Write()`, etc. Always `return` after writing a terminal response in a branch.

**Incorrect (what's wrong):**

```go
func handler(w http.ResponseWriter, r *http.Request) {
	err := process(r)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		// Missing return — falls through to success response
	}
	w.Write([]byte("success"))
}
```

**Correct (what's right):**

```go
func handler(w http.ResponseWriter, r *http.Request) {
	err := process(r)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("success"))
}
```
