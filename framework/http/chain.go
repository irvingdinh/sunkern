package http

import httpstd "net/http"

// Chain combines multiple middleware into a single Middleware. Middleware
// executes in the order given: the first argument sees the request first
// (outermost), the last argument runs closest to the handler (innermost).
//
//	protected := sunkernhttp.Chain(
//	    middleware.Auth(secret),
//	    middleware.RequireRole("admin"),
//	    middleware.Timeout(5 * time.Second),
//	)
//	api := server.Group("/api/admin")
//	api.Use(protected)
//
// Chain with zero arguments returns an identity middleware (no-op wrapper).
// Chain with one argument returns that middleware unchanged.
func Chain(mw ...Middleware) Middleware {
	switch len(mw) {
	case 0:
		return func(next httpstd.Handler) httpstd.Handler { return next }
	case 1:
		return mw[0]
	default:
		return func(next httpstd.Handler) httpstd.Handler {
			for i := len(mw) - 1; i >= 0; i-- {
				next = mw[i](next)
			}
			return next
		}
	}
}

// SkipIf wraps a middleware so that it is bypassed when the skip function
// returns true for the incoming request. When skipped, the request goes
// directly to the next handler without entering the middleware.
//
//	auth := middleware.Auth(secret)
//	auth = sunkernhttp.SkipIf(auth, func(r *http.Request) bool {
//	    return r.URL.Path == "/health"
//	})
func SkipIf(mw Middleware, skip func(r *httpstd.Request) bool) Middleware {
	return func(next httpstd.Handler) httpstd.Handler {
		wrapped := mw(next)
		return httpstd.HandlerFunc(func(w httpstd.ResponseWriter, r *httpstd.Request) {
			if skip(r) {
				next.ServeHTTP(w, r)
				return
			}
			wrapped.ServeHTTP(w, r)
		})
	}
}

// SkipMethods wraps a middleware so that it is bypassed for requests whose
// HTTP method matches any of the given methods. This is a convenience
// wrapper around [SkipIf] for the common case of exempting safe methods
// from CSRF checks or skipping auth on OPTIONS preflight.
//
//	csrf := middleware.CSRF(secret)
//	csrf = sunkernhttp.SkipMethods(csrf, "GET", "HEAD", "OPTIONS")
func SkipMethods(mw Middleware, methods ...string) Middleware {
	if len(methods) == 0 {
		return mw
	}
	set := make(map[string]struct{}, len(methods))
	for _, m := range methods {
		set[m] = struct{}{}
	}
	return SkipIf(mw, func(r *httpstd.Request) bool {
		_, ok := set[r.Method]
		return ok
	})
}

// SkipPaths wraps a middleware so that it is bypassed for requests whose
// URL path exactly matches any of the given paths. This is a convenience
// wrapper around [SkipIf] for the common case of excluding health checks,
// metrics endpoints, or public routes from authentication.
//
//	auth := middleware.Auth(secret)
//	auth = sunkernhttp.SkipPaths(auth, "/health", "/metrics", "/api/public/status")
func SkipPaths(mw Middleware, paths ...string) Middleware {
	if len(paths) == 0 {
		return mw
	}
	set := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		set[p] = struct{}{}
	}
	return SkipIf(mw, func(r *httpstd.Request) bool {
		_, ok := set[r.URL.Path]
		return ok
	})
}
