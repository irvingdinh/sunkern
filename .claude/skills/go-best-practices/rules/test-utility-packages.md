---
title: Use httptest and iotest Utility Packages
impact: LOW-MEDIUM
impactDescription: simplifies HTTP and I/O tests
tags: testing, httptest, iotest, utilities
---

## Use httptest and iotest Utility Packages

**Impact: LOW-MEDIUM (simplifies HTTP and I/O tests)**

Go provides purpose-built testing utilities in the standard library. `net/http/httptest` creates test HTTP servers and recorders without binding to real network ports. `testing/iotest` provides readers that simulate errors, partial reads, and other edge cases.

Using these avoids the complexity and flakiness of starting real servers, picking random ports, and managing network-dependent test infrastructure.

**Incorrect (what's wrong):**

```go
func TestHandler(t *testing.T) {
	// Starts a real server — port conflicts, cleanup issues, slow
	go http.ListenAndServe(":0", handler)
	time.Sleep(100 * time.Millisecond) // Wait for server to start
	resp, _ := http.Get("http://localhost:???/path")
	// ...
}

func TestProcessReader(t *testing.T) {
	// Only tests the happy path — no error simulation
	r := strings.NewReader("valid data")
	result, err := processReader(r)
	// What about read errors? Partial reads? EOF?
}
```

**Correct (what's right):**

```go
// httptest.NewServer: full HTTP server without port management
func TestHandler(t *testing.T) {
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/path")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	// Assert on resp.StatusCode, body, headers...
}

// httptest.NewRecorder: test handlers without any server
func TestHandlerDirect(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
}

// iotest: simulate read errors
func TestProcessReaderError(t *testing.T) {
	r := iotest.ErrReader(errors.New("disk failure"))
	_, err := processReader(r)
	if err == nil {
		t.Error("expected error from failing reader")
	}
}

// iotest: simulate partial reads (one byte at a time)
func TestProcessReaderSlow(t *testing.T) {
	r := iotest.OneByteReader(strings.NewReader("valid data"))
	result, err := processReader(r)
	if err != nil {
		t.Fatalf("should handle slow reader: %v", err)
	}
	// Assert result...
}
```
