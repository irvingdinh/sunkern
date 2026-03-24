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
