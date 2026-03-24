package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	sunkernlog "sunkern.local/framework/log"
)

// ---------------------------------------------------------------------------
// responseRecorder
// ---------------------------------------------------------------------------

// responseRecorder wraps http.ResponseWriter to capture the status code and
// bytes written for logging.
type responseRecorder struct {
	http.ResponseWriter
	status       int
	bytesWritten int
	wroteHeader  bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytesWritten += n
	return n, err
}

// Flush implements http.Flusher by delegating to the underlying writer if
// it supports it.
func (r *responseRecorder) Flush() {
	if fl, ok := r.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter. Required for
// http.ResponseController compatibility (Go 1.20+).
func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// ---------------------------------------------------------------------------
// RequestID
// ---------------------------------------------------------------------------

var reqCounter atomic.Uint64

// RequestID is an HTTP middleware that assigns a unique request ID to each
// request. If the incoming request has an X-Request-Id header, that value
// is used; otherwise a new ID is generated. The ID is stored in the request
// context and set as a response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			rid = fmt.Sprintf("req-%d", reqCounter.Add(1))
		} else if len(rid) > 36 {
			rid = rid[:36]
		}

		ctx := sunkernlog.WithRequestID(r.Context(), rid)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-Id", rid)

		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// RequestLogger
// ---------------------------------------------------------------------------

// RequestLogger is an HTTP middleware that logs each request with structured
// fields and smart log level selection based on response status code.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start)

		level := slog.LevelInfo
		switch {
		case rec.status >= 500:
			level = slog.LevelError
		case rec.status >= 400:
			level = slog.LevelWarn
		}

		slog.LogAttrs(r.Context(), level, "http request",
			slog.String("log_type", "http_request"),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Float64("latency_ms", float64(elapsed.Nanoseconds())/1e6),
			slog.String("client_ip", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
			slog.Int("response_size", rec.bytesWritten),
		)
	})
}
