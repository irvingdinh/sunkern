package http

import (
	httpstd "net/http"
	"net/http/httptest"
	"testing"
)

// headerMiddleware returns a Middleware that sets a response header.
func headerMiddleware(key, value string) Middleware {
	return func(next httpstd.Handler) httpstd.Handler {
		return httpstd.HandlerFunc(func(w httpstd.ResponseWriter, r *httpstd.Request) {
			w.Header().Set(key, value)
			next.ServeHTTP(w, r)
		})
	}
}

// appendMiddleware returns a Middleware that appends a value to a response
// header, used to verify execution order.
func appendMiddleware(value string) Middleware {
	return func(next httpstd.Handler) httpstd.Handler {
		return httpstd.HandlerFunc(func(w httpstd.ResponseWriter, r *httpstd.Request) {
			w.Header().Add("X-Order", value)
			next.ServeHTTP(w, r)
		})
	}
}

func noop(w httpstd.ResponseWriter, _ *httpstd.Request) {
	w.WriteHeader(httpstd.StatusOK)
}

// ---------------------------------------------------------------------------
// Chain
// ---------------------------------------------------------------------------

func TestChainEmpty(t *testing.T) {
	c := Chain()
	handler := c(httpstd.HandlerFunc(noop))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestChainSingle(t *testing.T) {
	c := Chain(headerMiddleware("X-Test", "one"))
	handler := c(httpstd.HandlerFunc(noop))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if v := rec.Header().Get("X-Test"); v != "one" {
		t.Errorf("X-Test = %q, want %q", v, "one")
	}
}

func TestChainOrder(t *testing.T) {
	// First middleware in Chain should run first (outermost).
	c := Chain(
		appendMiddleware("A"),
		appendMiddleware("B"),
		appendMiddleware("C"),
	)
	handler := c(httpstd.HandlerFunc(func(w httpstd.ResponseWriter, _ *httpstd.Request) {
		w.Header().Add("X-Order", "handler")
		w.WriteHeader(200)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	order := rec.Header().Values("X-Order")
	want := []string{"A", "B", "C", "handler"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestChainComposable(t *testing.T) {
	// Chain of chains should compose correctly.
	inner := Chain(appendMiddleware("B"), appendMiddleware("C"))
	outer := Chain(appendMiddleware("A"), inner)

	handler := outer(httpstd.HandlerFunc(func(w httpstd.ResponseWriter, _ *httpstd.Request) {
		w.Header().Add("X-Order", "handler")
		w.WriteHeader(200)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	order := rec.Header().Values("X-Order")
	want := []string{"A", "B", "C", "handler"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// SkipIf
// ---------------------------------------------------------------------------

func TestSkipIfTrue(t *testing.T) {
	mw := headerMiddleware("X-Auth", "checked")
	skipped := SkipIf(mw, func(r *httpstd.Request) bool {
		return r.URL.Path == "/health"
	})

	handler := skipped(httpstd.HandlerFunc(noop))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))

	if v := rec.Header().Get("X-Auth"); v != "" {
		t.Errorf("X-Auth = %q, want empty (middleware should be skipped)", v)
	}
}

func TestSkipIfFalse(t *testing.T) {
	mw := headerMiddleware("X-Auth", "checked")
	skipped := SkipIf(mw, func(r *httpstd.Request) bool {
		return r.URL.Path == "/health"
	})

	handler := skipped(httpstd.HandlerFunc(noop))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/users", nil))

	if v := rec.Header().Get("X-Auth"); v != "checked" {
		t.Errorf("X-Auth = %q, want %q", v, "checked")
	}
}

// ---------------------------------------------------------------------------
// SkipPaths
// ---------------------------------------------------------------------------

func TestSkipPathsMatched(t *testing.T) {
	mw := headerMiddleware("X-Auth", "checked")
	skipped := SkipPaths(mw, "/health", "/metrics")

	handler := skipped(httpstd.HandlerFunc(noop))

	for _, path := range []string{"/health", "/metrics"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))

		if v := rec.Header().Get("X-Auth"); v != "" {
			t.Errorf("path %s: X-Auth = %q, want empty", path, v)
		}
	}
}

func TestSkipPathsNotMatched(t *testing.T) {
	mw := headerMiddleware("X-Auth", "checked")
	skipped := SkipPaths(mw, "/health", "/metrics")

	handler := skipped(httpstd.HandlerFunc(noop))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/data", nil))

	if v := rec.Header().Get("X-Auth"); v != "checked" {
		t.Errorf("X-Auth = %q, want %q", v, "checked")
	}
}

func TestSkipPathsEmpty(t *testing.T) {
	called := false
	mw := func(next httpstd.Handler) httpstd.Handler {
		return httpstd.HandlerFunc(func(w httpstd.ResponseWriter, r *httpstd.Request) {
			called = true
			next.ServeHTTP(w, r)
		})
	}

	// SkipPaths with no paths should return the original middleware.
	skipped := SkipPaths(Middleware(mw))

	handler := skipped(httpstd.HandlerFunc(noop))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/anything", nil))

	if !called {
		t.Error("middleware should have been called (no paths to skip)")
	}
}

func TestSkipPathsExactMatch(t *testing.T) {
	mw := headerMiddleware("X-Auth", "checked")
	skipped := SkipPaths(mw, "/health")

	handler := skipped(httpstd.HandlerFunc(noop))

	// /health/deep should NOT be skipped (not exact match).
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/health/deep", nil))

	if v := rec.Header().Get("X-Auth"); v != "checked" {
		t.Errorf("X-Auth = %q, want %q (partial path should not skip)", v, "checked")
	}
}

// ---------------------------------------------------------------------------
// Chain + SkipPaths composition
// ---------------------------------------------------------------------------

func TestChainWithSkipPaths(t *testing.T) {
	auth := headerMiddleware("X-Auth", "checked")
	auth = SkipPaths(auth, "/health")

	c := Chain(
		appendMiddleware("logging"),
		auth,
	)

	handler := c(httpstd.HandlerFunc(func(w httpstd.ResponseWriter, _ *httpstd.Request) {
		w.Header().Add("X-Order", "handler")
		w.WriteHeader(200)
	}))

	// /health: logging runs, auth skipped.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))

	order := rec.Header().Values("X-Order")
	if len(order) != 2 || order[0] != "logging" || order[1] != "handler" {
		t.Errorf("/health order = %v, want [logging handler]", order)
	}
	if v := rec.Header().Get("X-Auth"); v != "" {
		t.Errorf("/health X-Auth = %q, want empty", v)
	}

	// /api: both run.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api", nil))

	if v := rec.Header().Get("X-Auth"); v != "checked" {
		t.Errorf("/api X-Auth = %q, want %q", v, "checked")
	}
}
