---
title: Configure HTTP Client and Server Timeouts
impact: HIGH
impactDescription: prevents hanging connections
tags: http, client, server, timeouts
---

## Configure HTTP Client and Server Timeouts

**Impact: HIGH (prevents hanging connections)**

The default `http.Client` has no timeout. A request to a slow or unresponsive server will block the goroutine forever, eventually exhausting resources. Similarly, the default `http.Server` has no read, write, or idle timeouts, making it vulnerable to slowloris attacks and connection exhaustion.

Never use `http.Get()`, `http.Post()`, or `http.DefaultClient` in production. Always create a configured client and server with explicit timeouts.

**Incorrect (what's wrong):**

```go
// Client: no timeout — can block forever
resp, err := http.Get(url)

// Server: no timeouts — vulnerable to slow clients
http.ListenAndServe(":8080", handler)
```

**Correct (what's right):**

```go
// Client: explicit timeout
client := &http.Client{
	Timeout: 10 * time.Second,
}
resp, err := client.Get(url)

// For finer control, configure the transport
client := &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		TLSHandshakeTimeout:  5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		IdleConnTimeout:      90 * time.Second,
	},
}

// Server: explicit timeouts
srv := &http.Server{
	Addr:         ":8080",
	Handler:      handler,
	ReadTimeout:  5 * time.Second,
	WriteTimeout: 10 * time.Second,
	IdleTimeout:  120 * time.Second,
}
srv.ListenAndServe()
```
