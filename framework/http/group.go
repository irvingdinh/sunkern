package http

import (
	httpstd "net/http"
	"strings"
)

// Middleware is a function that wraps an HTTP handler to add cross-cutting
// behavior (auth, logging, rate limiting, etc.). It follows the standard
// Go middleware pattern.
type Middleware func(httpstd.Handler) httpstd.Handler

// RouteGroup represents a group of routes sharing a common URL prefix and
// middleware stack. Create groups via Server.Group. Groups can be nested
// arbitrarily deep.
//
//	api := server.Group("/api")
//	api.Use(authMiddleware)
//
//	users := api.Group("/users")
//	users.HandleFunc("GET /", uc.list)
//	users.HandleFunc("GET /{id}", uc.show)
//	users.HandleFunc("POST /", uc.create)
type RouteGroup struct {
	prefix     string
	mux        *httpstd.ServeMux
	middleware []Middleware
}

// Use appends middleware to this group. Middleware is applied to all routes
// registered on this group and on sub-groups created after this call.
// Middleware runs in the order added (first added = outermost).
func (g *RouteGroup) Use(mw ...Middleware) {
	g.middleware = append(g.middleware, mw...)
}

// Group creates a sub-group with the given prefix appended to this group's
// prefix. The sub-group inherits a snapshot of the parent's current
// middleware stack.
func (g *RouteGroup) Group(prefix string) *RouteGroup {
	mw := make([]Middleware, len(g.middleware))
	copy(mw, g.middleware)
	return &RouteGroup{
		prefix:     g.prefix + prefix,
		mux:        g.mux,
		middleware: mw,
	}
}

// Handle registers a handler for the given pattern. The pattern uses Go
// 1.22+ ServeMux syntax (e.g., "GET /{id}", "POST /"). The group's prefix
// is prepended to the path portion.
//
// The path "/" is treated as the group root and maps to the exact prefix
// (no trailing slash). Use "/{$}" to match the prefix with a trailing
// slash exactly.
func (g *RouteGroup) Handle(pattern string, handler httpstd.Handler) {
	fullPattern := g.buildPattern(pattern)
	g.mux.Handle(fullPattern, g.applyMiddleware(handler))
}

// HandleFunc registers a handler function. See Handle for pattern syntax.
func (g *RouteGroup) HandleFunc(pattern string, handler httpstd.HandlerFunc) {
	g.Handle(pattern, handler)
}

// buildPattern prepends the group prefix to the path portion of a ServeMux
// pattern. "GET /users" with prefix "/api" becomes "GET /api/users".
// The special path "/" maps to the prefix itself (no trailing slash).
func (g *RouteGroup) buildPattern(pattern string) string {
	method, path := splitPattern(pattern)
	fullPath := g.prefix + path
	if path == "/" {
		fullPath = g.prefix
	}
	if method == "" {
		return fullPath
	}
	return method + " " + fullPath
}

// applyMiddleware wraps the handler with the group's middleware stack.
// First middleware added runs outermost (sees the request first).
func (g *RouteGroup) applyMiddleware(h httpstd.Handler) httpstd.Handler {
	for i := len(g.middleware) - 1; i >= 0; i-- {
		h = g.middleware[i](h)
	}
	return h
}

// splitPattern splits a ServeMux pattern into method and path components.
// "GET /foo" → ("GET", "/foo"), "/foo" → ("", "/foo").
func splitPattern(pattern string) (method, path string) {
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		return pattern[:i], pattern[i+1:]
	}
	return "", pattern
}
