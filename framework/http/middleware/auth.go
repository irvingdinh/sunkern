package middleware

import (
	"encoding/json"
	"net/http"

	sunkernlog "sunkern.local/framework/log"
)

// Auth returns middleware that validates JWT Bearer tokens from the
// Authorization header. On success it stores the decoded [Claims] in
// the request context (accessible via [ClaimsFromCtx]) and sets the
// user_id for structured logging (via [sunkernlog.WithUserID]).
//
// Returns 401 Unauthorized if the header is missing, malformed, or the
// token is invalid/expired.
//
//	secret := []byte(config.Get[string]("jwt.secret"))
//	api := server.Group("/api")
//	api.Use(middleware.Auth(secret))
func Auth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if len(auth) < 8 || auth[:7] != "Bearer " {
				writeUnauthorized(w, "Missing or invalid authorization header")
				return
			}

			claims, err := VerifyToken(secret, auth[7:])
			if err != nil {
				msg := "Invalid or expired token"
				if err == ErrTokenExpired {
					msg = "Token expired"
				}
				writeUnauthorized(w, msg)
				return
			}

			ctx := WithClaims(r.Context(), claims)
			if claims.Subject != "" {
				ctx = sunkernlog.WithUserID(ctx, claims.Subject)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns middleware that checks the authenticated user's
// role against the allowed set. Returns 403 Forbidden if the role does
// not match. Must be applied after [Auth] middleware.
//
//	admin := api.Group("/admin")
//	admin.Use(middleware.RequireRole("super_admin", "admin"))
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromCtx(r.Context())
			if claims == nil {
				writeUnauthorized(w, "Authentication required")
				return
			}

			if _, ok := allowed[claims.Role]; !ok {
				writeForbidden(w, "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ---------------------------------------------------------------------------
// Error response helpers (inline JSON to avoid circular import with
// parent http package — same pattern as recover.go and ratelimit.go)
// ---------------------------------------------------------------------------

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    "unauthorized",
			"message": msg,
		},
	})
}

func writeForbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    "forbidden",
			"message": msg,
		},
	})
}
