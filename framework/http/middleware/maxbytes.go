package middleware

import (
	"net/http"
)

// MaxBytes returns middleware that limits the request body size to the
// given number of bytes. When a handler (or [Bind], [BindForm]) reads
// past the limit, the read fails and the framework translates it to a
// 413 Payload Too Large response.
//
// Apply this middleware to route groups that accept request bodies to
// prevent oversized payloads from consuming memory before the handler
// can reject them.
//
//	api := server.Group("/api")
//	api.Use(middleware.MaxBytes(1 << 20)) // 1 MiB
func MaxBytes(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}
