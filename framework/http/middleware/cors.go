package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSOption configures the CORS middleware.
type CORSOption func(*corsConfig)

type corsConfig struct {
	allowOrigins     []string
	allowMethods     []string
	allowHeaders     []string
	exposeHeaders    []string
	allowCredentials bool
	maxAge           int
}

// WithAllowOrigins sets the allowed origins. Use "*" to allow all origins
// (cannot be combined with WithAllowCredentials).
// Default: empty — no cross-origin requests allowed.
func WithAllowOrigins(origins ...string) CORSOption {
	return func(c *corsConfig) { c.allowOrigins = origins }
}

// WithAllowMethods sets the HTTP methods allowed in preflight requests.
// Default: GET, POST, PUT, PATCH, DELETE, OPTIONS.
func WithAllowMethods(methods ...string) CORSOption {
	return func(c *corsConfig) { c.allowMethods = methods }
}

// WithAllowHeaders sets the request headers allowed in preflight requests.
// Default: Content-Type, Authorization.
func WithAllowHeaders(headers ...string) CORSOption {
	return func(c *corsConfig) { c.allowHeaders = headers }
}

// WithExposeHeaders sets the response headers that browsers may access.
// Default: none (only CORS-safelisted headers are exposed).
func WithExposeHeaders(headers ...string) CORSOption {
	return func(c *corsConfig) { c.exposeHeaders = headers }
}

// WithAllowCredentials enables credentials (cookies, authorization headers)
// in cross-origin requests. When true, the wildcard "*" origin is not used —
// the actual request origin is reflected instead.
// Default: false.
func WithAllowCredentials(allow bool) CORSOption {
	return func(c *corsConfig) { c.allowCredentials = allow }
}

// WithMaxAge sets how long (in seconds) browsers cache preflight results.
// Default: 86400 (24 hours).
func WithMaxAge(seconds int) CORSOption {
	return func(c *corsConfig) { c.maxAge = seconds }
}

// CORS returns a middleware that handles Cross-Origin Resource Sharing.
// It responds to preflight OPTIONS requests with 204 No Content and sets
// appropriate CORS headers on actual requests.
//
// Sunkern is same-origin by default (one binary serves everything), so
// CORS is restrictive unless origins are explicitly allowed. Enable it
// when the API serves a mobile app or external consumers.
//
//	api := server.Group("/api")
//	api.Use(middleware.CORS(
//	    middleware.WithAllowOrigins("https://app.example.com"),
//	    middleware.WithAllowCredentials(true),
//	))
func CORS(opts ...CORSOption) func(http.Handler) http.Handler {
	cfg := &corsConfig{
		allowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		allowHeaders: []string{"Content-Type", "Authorization"},
		maxAge:       86400,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Pre-join header values to avoid per-request allocation.
	methodsStr := strings.Join(cfg.allowMethods, ", ")
	headersStr := strings.Join(cfg.allowHeaders, ", ")
	exposeStr := strings.Join(cfg.exposeHeaders, ", ")
	maxAgeStr := strconv.Itoa(cfg.maxAge)

	allowAll := len(cfg.allowOrigins) == 1 && cfg.allowOrigins[0] == "*"

	// Build origin set for O(1) lookup.
	originSet := make(map[string]struct{}, len(cfg.allowOrigins))
	for _, o := range cfg.allowOrigins {
		originSet[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				// Not a cross-origin request.
				next.ServeHTTP(w, r)
				return
			}

			// Check if origin is allowed.
			allowed := allowAll
			if !allowed {
				_, allowed = originSet[origin]
			}
			if !allowed {
				// Origin not allowed — proceed without CORS headers.
				next.ServeHTTP(w, r)
				return
			}

			h := w.Header()

			// Per the CORS spec, Access-Control-Allow-Origin must be
			// the literal origin (not "*") when credentials are enabled.
			if allowAll && !cfg.allowCredentials {
				h.Set("Access-Control-Allow-Origin", "*")
			} else {
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
			}

			if cfg.allowCredentials {
				h.Set("Access-Control-Allow-Credentials", "true")
			}

			if exposeStr != "" {
				h.Set("Access-Control-Expose-Headers", exposeStr)
			}

			// Preflight request — respond and stop the chain.
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				h.Set("Access-Control-Allow-Methods", methodsStr)
				h.Set("Access-Control-Allow-Headers", headersStr)
				h.Set("Access-Control-Max-Age", maxAgeStr)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
