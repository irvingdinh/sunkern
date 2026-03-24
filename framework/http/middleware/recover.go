package middleware

import (
	"log/slog"
	"net/http"
	"runtime"
)

// Recover is an HTTP middleware that catches panics from downstream handlers,
// logs the error with a stack trace, and returns a 500 Internal Server Error
// JSON response. Without this middleware, a single panic in any handler would
// crash the entire server process.
//
// Place Recover inside RequestLogger so that panicked requests are still
// logged with the correct 500 status code:
//
//	handler = middleware.Recover(handler)     // innermost
//	handler = middleware.RequestLogger(handler)
//	handler = middleware.RequestID(handler)   // outermost
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)

				slog.ErrorContext(r.Context(), "http: panic recovered",
					"error", err,
					"stack", string(buf[:n]),
				)

				// Write a JSON error response. If headers were already
				// sent (partial streaming response), WriteHeader is a
				// no-op and the client receives a truncated response —
				// acceptable since the panic is already logged.
				writeErrorJSON(w, http.StatusInternalServerError, "internal_error", "Internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
