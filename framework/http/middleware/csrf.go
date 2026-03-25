package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

// CSRFOption configures the CSRF middleware.
type CSRFOption func(*csrfConfig)

type csrfConfig struct {
	cookieName string
	headerName string
	cookiePath string
	secure     bool
	sameSite   http.SameSite
}

// WithCSRFCookieName sets the name of the CSRF cookie.
// Default: "_csrf".
func WithCSRFCookieName(name string) CSRFOption {
	return func(c *csrfConfig) { c.cookieName = name }
}

// WithCSRFHeaderName sets the request header that must contain the CSRF token.
// Default: "X-CSRF-Token".
func WithCSRFHeaderName(name string) CSRFOption {
	return func(c *csrfConfig) { c.headerName = name }
}

// WithCSRFCookiePath sets the Path attribute of the CSRF cookie.
// Default: "/".
func WithCSRFCookiePath(path string) CSRFOption {
	return func(c *csrfConfig) { c.cookiePath = path }
}

// WithCSRFSecure sets the Secure attribute of the CSRF cookie.
// When true the cookie is only sent over HTTPS.
// Default: true.
func WithCSRFSecure(secure bool) CSRFOption {
	return func(c *csrfConfig) { c.secure = secure }
}

// WithCSRFSameSite sets the SameSite attribute of the CSRF cookie.
// Default: [http.SameSiteLaxMode].
func WithCSRFSameSite(ss http.SameSite) CSRFOption {
	return func(c *csrfConfig) { c.sameSite = ss }
}

// CSRF returns middleware that protects against Cross-Site Request Forgery
// using the double-submit cookie pattern. On every request it ensures a
// CSRF cookie is set (generating one if absent). For state-changing methods
// (POST, PUT, PATCH, DELETE) it requires the same token in the request
// header and rejects mismatches with 403 Forbidden.
//
// The client-side flow:
//  1. Read the CSRF cookie value (e.g., document.cookie)
//  2. Include it as a request header on every state-changing request
//
// Safe methods (GET, HEAD, OPTIONS, TRACE) never require the header,
// so this middleware is commonly applied globally without SkipMethods.
//
//	api := server.Group("/api")
//	api.Use(middleware.CSRF(
//	    middleware.WithCSRFSecure(false), // development only
//	))
func CSRF(opts ...CSRFOption) func(http.Handler) http.Handler {
	cfg := &csrfConfig{
		cookieName: "_csrf",
		headerName: "X-CSRF-Token",
		cookiePath: "/",
		secure:     true,
		sameSite:   http.SameSiteLaxMode,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	safeMethods := map[string]struct{}{
		http.MethodGet:     {},
		http.MethodHead:    {},
		http.MethodOptions: {},
		http.MethodTrace:   {},
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read existing cookie or generate a new token.
			cookie, err := r.Cookie(cfg.cookieName)
			token := ""
			if err == nil && len(cookie.Value) == 64 {
				token = cookie.Value
			}
			if token == "" {
				token, err = generateCSRFToken()
				if err != nil {
					writeInternalError(w)
					return
				}
			}

			// Always set the cookie to refresh MaxAge and ensure it
			// exists for new sessions.
			http.SetCookie(w, &http.Cookie{
				Name:     cfg.cookieName,
				Value:    token,
				Path:     cfg.cookiePath,
				HttpOnly: false, // JS must be able to read it
				Secure:   cfg.secure,
				SameSite: cfg.sameSite,
			})

			// Safe methods never need the header.
			if _, safe := safeMethods[r.Method]; safe {
				next.ServeHTTP(w, r)
				return
			}

			// State-changing method — require matching header.
			header := r.Header.Get(cfg.headerName)
			if header == "" {
				writeForbidden(w, "Missing CSRF token")
				return
			}

			if subtle.ConstantTimeCompare([]byte(token), []byte(header)) != 1 {
				writeForbidden(w, "Invalid CSRF token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// generateCSRFToken returns 32 random bytes as a 64-character hex string.
func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
