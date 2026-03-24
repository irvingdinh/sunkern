package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	sunkernlog "sunkern.local/framework/log"
)

// TokenLookup resolves an opaque API token to claims. Implementations
// typically query the database for the token record.
//
// Return semantics:
//   - (*Claims, nil) — token is valid, proceed with these claims
//   - (nil, nil) — token not found, middleware returns 401
//   - (nil, error) — lookup failed, middleware returns 500
type TokenLookup func(ctx context.Context, token string) (*Claims, error)

// APIToken returns middleware that validates opaque Bearer tokens by
// calling the provided lookup function. On success it stores the
// decoded [Claims] in the request context (same key as [Auth]) and
// sets the user_id for structured logging.
//
// Use this for long-lived, revocable API tokens stored in the database.
// For short-lived, self-contained tokens, use [Auth] (JWT) instead.
//
//	lookup := func(ctx context.Context, token string) (*middleware.Claims, error) {
//	    // query database for token record
//	    return &middleware.Claims{Subject: "user-123", Role: "user"}, nil
//	}
//	api := server.Group("/api")
//	api.Use(middleware.APIToken(lookup))
func APIToken(lookup TokenLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				writeUnauthorized(w, "Missing or invalid authorization header")
				return
			}

			claims, err := lookup(r.Context(), token)
			if err != nil {
				slog.ErrorContext(r.Context(), "apitoken: lookup failed",
					"error", err,
				)
				writeInternalError(w)
				return
			}
			if claims == nil {
				writeUnauthorized(w, "Invalid API token")
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

// GenerateToken creates a cryptographically random opaque token string.
// The token is 32 random bytes encoded as 64 hex characters. Use this
// when creating new API tokens to store in the database.
//
//	token, err := middleware.GenerateToken()
//	// store token (or its hash) in the database
//	// return token to the user (they won't see it again)
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

// extractBearerToken extracts the token from "Bearer <token>" in the
// Authorization header. Returns empty string if the header is missing
// or malformed.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) < 8 {
		return ""
	}
	// Case-insensitive "Bearer " prefix per RFC 6750.
	if !strings.EqualFold(auth[:7], "Bearer ") {
		return ""
	}
	return auth[7:]
}

func writeInternalError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    "internal_error",
			"message": "Internal server error",
		},
	})
}
