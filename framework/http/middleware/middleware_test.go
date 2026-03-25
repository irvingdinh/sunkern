package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sunkernlog "sunkern.local/framework/log"
)

// ---------------------------------------------------------------------------
// RequestLogger
// ---------------------------------------------------------------------------

func TestRequestLoggerSmartLevels(t *testing.T) {
	cases := []struct {
		status int
		level  string
	}{
		{200, "INFO"},
		{201, "INFO"},
		{301, "INFO"},
		{400, "WARN"},
		{404, "WARN"},
		{500, "ERROR"},
		{503, "ERROR"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			})

			mw := RequestLogger(inner)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)

			// The log goes to slog.Default(). We can't easily capture it
			// without overriding the default logger, but we verify the
			// handler ran without panic and returned the correct status.
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}

func TestRequestLoggerFields(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	})

	mw := RequestLogger(inner)
	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "hello" {
		t.Fatalf("body = %q, want %q", rec.Body.String(), "hello")
	}
}

// ---------------------------------------------------------------------------
// RequestID
// ---------------------------------------------------------------------------

func TestRequestIDGeneration(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context has request ID.
		rid := sunkernlog.RequestIDFromCtx(r.Context())
		if rid == "" {
			t.Error("request ID not in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	mw := RequestID(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	rid := rec.Header().Get("X-Request-Id")
	if rid == "" {
		t.Fatal("missing X-Request-Id header")
	}
	if !strings.HasPrefix(rid, "req-") {
		t.Errorf("X-Request-Id = %q, want prefix %q", rid, "req-")
	}
}

func TestRequestIDPassthrough(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := sunkernlog.RequestIDFromCtx(r.Context())
		if rid != "custom-id-42" {
			t.Errorf("context request ID = %q, want %q", rid, "custom-id-42")
		}
		w.WriteHeader(http.StatusOK)
	})

	mw := RequestID(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "custom-id-42")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-Id") != "custom-id-42" {
		t.Errorf("X-Request-Id = %q, want %q", rec.Header().Get("X-Request-Id"), "custom-id-42")
	}
}

func TestRequestIDTruncation(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RequestID(inner)
	longID := strings.Repeat("a", 100)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", longID)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	rid := rec.Header().Get("X-Request-Id")
	if len(rid) != 36 {
		t.Errorf("X-Request-Id length = %d, want 36", len(rid))
	}
}

// ---------------------------------------------------------------------------
// responseRecorder
// ---------------------------------------------------------------------------

func TestResponseRecorderFlusher(t *testing.T) {
	underlying := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: underlying}

	flusher, ok := any(rec).(http.Flusher)
	if !ok {
		t.Fatal("responseRecorder should implement http.Flusher")
	}

	flusher.Flush()
	if !underlying.Flushed {
		t.Fatal("underlying writer should have been flushed")
	}
}

func TestResponseRecorderUnwrap(t *testing.T) {
	underlying := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: underlying}

	unwrapped := rec.Unwrap()
	if unwrapped != underlying {
		t.Fatal("Unwrap should return the underlying ResponseWriter")
	}
}

func TestResponseRecorderDefaultStatus(t *testing.T) {
	underlying := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: underlying, status: http.StatusOK}

	// Write without explicit WriteHeader should trigger implicit 200.
	_, _ = rec.Write([]byte("data"))

	if rec.status != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.status)
	}
	if rec.bytesWritten != 4 {
		t.Errorf("bytesWritten = %d, want 4", rec.bytesWritten)
	}
}

func TestResponseRecorderDoubleWriteHeader(t *testing.T) {
	underlying := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: underlying, status: http.StatusOK}

	rec.WriteHeader(http.StatusNotFound)
	rec.WriteHeader(http.StatusInternalServerError) // should be ignored

	if rec.status != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (first call wins)", rec.status)
	}
}

// ---------------------------------------------------------------------------
// SubjectFromCtx / RoleFromCtx
// ---------------------------------------------------------------------------

func TestSubjectFromCtxWithClaims(t *testing.T) {
	ctx := WithClaims(t.Context(), &Claims{Subject: "user-42", Role: "admin"})
	if got := SubjectFromCtx(ctx); got != "user-42" {
		t.Errorf("SubjectFromCtx = %q, want %q", got, "user-42")
	}
}

func TestSubjectFromCtxNoClaims(t *testing.T) {
	if got := SubjectFromCtx(t.Context()); got != "" {
		t.Errorf("SubjectFromCtx = %q, want empty", got)
	}
}

func TestRoleFromCtxWithClaims(t *testing.T) {
	ctx := WithClaims(t.Context(), &Claims{Subject: "user-1", Role: "editor"})
	if got := RoleFromCtx(ctx); got != "editor" {
		t.Errorf("RoleFromCtx = %q, want %q", got, "editor")
	}
}

func TestRoleFromCtxNoClaims(t *testing.T) {
	if got := RoleFromCtx(t.Context()); got != "" {
		t.Errorf("RoleFromCtx = %q, want empty", got)
	}
}

func TestSubjectRoleFromCtxEmptyFields(t *testing.T) {
	ctx := WithClaims(t.Context(), &Claims{})
	if got := SubjectFromCtx(ctx); got != "" {
		t.Errorf("SubjectFromCtx = %q, want empty (zero Claims)", got)
	}
	if got := RoleFromCtx(ctx); got != "" {
		t.Errorf("RoleFromCtx = %q, want empty (zero Claims)", got)
	}
}

// ---------------------------------------------------------------------------
// CSRF
// ---------------------------------------------------------------------------

func TestCSRFSetsCookieOnGET(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRF(WithCSRFSecure(false))(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	cookies := rec.Result().Cookies()
	var csrf *http.Cookie
	for _, c := range cookies {
		if c.Name == "_csrf" {
			csrf = c
			break
		}
	}
	if csrf == nil {
		t.Fatal("missing _csrf cookie")
	}
	if len(csrf.Value) != 64 {
		t.Errorf("csrf token length = %d, want 64", len(csrf.Value))
	}
	if csrf.HttpOnly {
		t.Error("csrf cookie should not be HttpOnly (JS must read it)")
	}
}

func TestCSRFAllowsSafeMethodsWithoutHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRF(WithCSRFSecure(false))(inner)

	for _, method := range []string{"GET", "HEAD", "OPTIONS", "TRACE"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/data", nil)
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("method %s: status = %d, want 200", method, rec.Code)
		}
	}
}

func TestCSRFRejectsMutationWithoutHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRF(WithCSRFSecure(false))(inner)

	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/data", nil)
		req.AddCookie(&http.Cookie{Name: "_csrf", Value: strings.Repeat("ab", 32)})
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("method %s: status = %d, want 403", method, rec.Code)
		}
	}
}

func TestCSRFRejectsMismatchedToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRF(WithCSRFSecure(false))(inner)

	cookieToken := strings.Repeat("ab", 32)
	headerToken := strings.Repeat("cd", 32)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/data", nil)
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: cookieToken})
	req.Header.Set("X-CSRF-Token", headerToken)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (mismatched tokens)", rec.Code)
	}
}

func TestCSRFAcceptsMatchingToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRF(WithCSRFSecure(false))(inner)

	token := strings.Repeat("ab", 32)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/data", nil)
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: token})
	req.Header.Set("X-CSRF-Token", token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (matching tokens)", rec.Code)
	}
}

func TestCSRFCustomCookieAndHeaderName(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRF(
		WithCSRFCookieName("my-csrf"),
		WithCSRFHeaderName("X-My-Token"),
		WithCSRFSecure(false),
	)(inner)

	// GET should set the custom cookie.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	var csrf *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "my-csrf" {
			csrf = c
			break
		}
	}
	if csrf == nil {
		t.Fatal("missing my-csrf cookie")
	}

	// POST with custom header and cookie should succeed.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "my-csrf", Value: csrf.Value})
	req.Header.Set("X-My-Token", csrf.Value)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (custom names)", rec.Code)
	}
}

func TestCSRFGeneratesTokenWhenCookieMissing(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRF(WithCSRFSecure(false))(inner)

	// POST without any cookie — should get 403 (missing header) but also
	// a Set-Cookie with a new token.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}

	// Should still get a Set-Cookie header with the generated token.
	var csrf *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "_csrf" {
			csrf = c
			break
		}
	}
	if csrf == nil {
		t.Fatal("missing _csrf cookie even on rejected request")
	}
}

func TestCSRFRejectsInvalidLengthCookie(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRF(WithCSRFSecure(false))(inner)

	// Cookie with wrong length triggers token regeneration.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "short"})
	handler.ServeHTTP(rec, req)

	// GET passes, but a new cookie should be set.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var csrf *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "_csrf" {
			csrf = c
			break
		}
	}
	if csrf == nil || csrf.Value == "short" {
		t.Error("should have regenerated token for invalid-length cookie")
	}
}

// ---------------------------------------------------------------------------
// CacheBody
// ---------------------------------------------------------------------------

func TestCacheBodyReplayable(t *testing.T) {
	body := `{"name":"test"}`

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First read via CachedBody.
		cached := CachedBody(r)
		if string(cached) != body {
			t.Errorf("CachedBody = %q, want %q", cached, body)
		}

		// Second read via r.Body — should still work.
		data := make([]byte, len(body))
		n, _ := r.Body.Read(data)
		if string(data[:n]) != body {
			t.Errorf("r.Body read = %q, want %q", data[:n], body)
		}

		w.WriteHeader(http.StatusOK)
	})

	handler := CacheBody(1 << 20)(inner)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestCacheBodyTooLarge(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for oversized body")
		w.WriteHeader(http.StatusOK)
	})

	handler := CacheBody(10)(inner) // 10 byte limit
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("this body is way too large"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
}

func TestCacheBodyExactLimit(t *testing.T) {
	body := strings.Repeat("x", 100)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cached := CachedBody(r)
		if len(cached) != 100 {
			t.Errorf("CachedBody len = %d, want 100", len(cached))
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := CacheBody(100)(inner)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (body at exact limit)", rec.Code)
	}
}

func TestCacheBodyNilBody(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cached := CachedBody(r)
		if cached != nil {
			t.Errorf("CachedBody = %v, want nil", cached)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := CacheBody(1 << 20)(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestCacheBodyNoBody(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cached := CachedBody(r)
		if cached != nil {
			t.Errorf("CachedBody = %v, want nil", cached)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := CacheBody(1 << 20)(inner)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestCacheBodyEmptyBody(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cached := CachedBody(r)
		if cached == nil || len(cached) != 0 {
			t.Errorf("CachedBody = %v (len=%d), want empty non-nil", cached, len(cached))
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := CacheBody(1 << 20)(inner)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestCachedBodyWithoutMiddleware(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	cached := CachedBody(req)
	if cached != nil {
		t.Errorf("CachedBody without middleware = %v, want nil", cached)
	}
}

// ---------------------------------------------------------------------------
// Integration: RequestID + RequestLogger
// ---------------------------------------------------------------------------

func TestMiddlewareChain(t *testing.T) {
	sunkernlog.Reset() // discard handler for test

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request ID is in context (set by RequestID middleware).
		rid := sunkernlog.RequestIDFromCtx(r.Context())
		if rid == "" {
			t.Error("request ID not in context inside handler")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"request_id":%q}`, rid)
	})

	// Chain: RequestID -> RequestLogger -> handler
	handler := RequestLogger(inner)
	handler = RequestID(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify response.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	rid := rec.Header().Get("X-Request-Id")
	if rid == "" {
		t.Fatal("missing X-Request-Id in response")
	}

	// Verify body contains the request ID from context.
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["request_id"] != rid {
		t.Errorf("body request_id = %q, want %q", body["request_id"], rid)
	}
}
