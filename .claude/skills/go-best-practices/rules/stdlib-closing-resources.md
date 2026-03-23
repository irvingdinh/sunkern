---
title: Always Close io.Closer Resources
impact: HIGH
impactDescription: prevents resource leaks
tags: resources, close, defer, io, http
---

## Always Close io.Closer Resources

**Impact: HIGH (prevents resource leaks)**

Types that implement `io.Closer` must be closed after use: HTTP response bodies, `sql.Rows`, `os.File`, `gzip.Reader`, and others. Failing to close them leaks file descriptors, connections, or memory. Use `defer` immediately after the error check to guarantee cleanup, even on panics or early returns.

For HTTP responses, an unclosed body prevents the underlying TCP connection from being reused by the connection pool, eventually exhausting available connections.

**Incorrect (what's wrong):**

```go
// HTTP response body not closed — connection leak
resp, err := http.Get(url)
if err != nil {
	return err
}
body, err := io.ReadAll(resp.Body)

// File not closed — file descriptor leak
f, err := os.Open(path)
if err != nil {
	return err
}
data, err := io.ReadAll(f)

// sql.Rows not closed — connection held open
rows, err := db.Query("SELECT id FROM users")
if err != nil {
	return err
}
for rows.Next() {
	// process rows
}
```

**Correct (what's right):**

```go
// HTTP: defer Close immediately after error check
resp, err := http.Get(url)
if err != nil {
	return err
}
defer resp.Body.Close()
body, err := io.ReadAll(resp.Body)

// File: defer Close immediately after error check
f, err := os.Open(path)
if err != nil {
	return err
}
defer f.Close()
data, err := io.ReadAll(f)

// sql.Rows: defer Close immediately after error check
rows, err := db.Query("SELECT id FROM users")
if err != nil {
	return err
}
defer rows.Close()
for rows.Next() {
	// process rows
}
```
