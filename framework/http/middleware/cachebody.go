package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

type contextKey int

const cachedBodyKey contextKey = 1

// CacheBody returns middleware that reads the request body into memory
// and replaces r.Body with a replayable reader. This allows downstream
// handlers and middleware to read the body multiple times (e.g., for
// logging, signature verification, or retry logic).
//
// The maxSize parameter limits how many bytes are buffered. If the body
// exceeds maxSize, the middleware returns 413 Payload Too Large without
// calling the next handler.
//
// The cached bytes are also stored in the request context and can be
// retrieved via [CachedBody].
//
// Requests with no body (nil or http.NoBody) pass through untouched.
//
//	api := server.Group("/api/webhooks")
//	api.Use(middleware.CacheBody(1 << 20)) // 1 MiB max
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    body := middleware.CachedBody(r)   // []byte, can be read multiple times
//	    raw, _ := io.ReadAll(r.Body)       // also works — body is replayable
//	}
func CacheBody(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil || r.Body == http.NoBody {
				next.ServeHTTP(w, r)
				return
			}

			// Read up to maxSize+1 to detect oversized bodies.
			data, err := io.ReadAll(io.LimitReader(r.Body, maxSize+1))
			r.Body.Close()

			if err != nil {
				writeErrorJSON(w, http.StatusBadRequest, "bad_request", "Failed to read request body")
				return
			}

			if int64(len(data)) > maxSize {
				writeErrorJSON(w, http.StatusRequestEntityTooLarge, "payload_too_large", "Request body too large")
				return
			}

			// Replace body with a replayable reader and store in context.
			r.Body = io.NopCloser(bytes.NewReader(data))
			ctx := context.WithValue(r.Context(), cachedBodyKey, data)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CachedBody returns the request body bytes cached by the [CacheBody]
// middleware. Returns nil if the middleware was not applied or the
// request had no body.
//
// The returned slice is shared — do not modify it.
func CachedBody(r *http.Request) []byte {
	if v := r.Context().Value(cachedBodyKey); v != nil {
		return v.([]byte)
	}
	return nil
}
