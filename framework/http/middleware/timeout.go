package middleware

import (
	"context"
	"net/http"
	"time"
)

// Timeout returns middleware that sets a deadline on the request context.
// Handlers and downstream operations (database queries, HTTP calls) that
// respect the context will abort when the deadline expires.
//
// This does NOT buffer or forcibly terminate the response like
// [http.TimeoutHandler] — it only sets the context deadline. The server's
// WriteTimeout acts as the hard backstop. Timeout provides per-route
// granularity on top of the server-wide limit.
//
// SSE endpoints should NOT use this middleware because they manage their
// own write deadlines via [http.ResponseController].
//
//	api := server.Group("/api")
//	api.Use(middleware.Timeout(5 * time.Second))
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
