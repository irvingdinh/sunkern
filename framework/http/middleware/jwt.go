package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// JWT errors returned by [SignToken] and [VerifyToken].
var (
	ErrTokenMalformed = errors.New("jwt: malformed token")
	ErrTokenExpired   = errors.New("jwt: token expired")
	ErrTokenInvalid   = errors.New("jwt: invalid signature")
	ErrSecretEmpty    = errors.New("jwt: empty secret")
)

// Claims represents the payload of a JWT token. Subject identifies the
// principal (usually a user ID). Role carries the authorization level.
// ExpiresAt and IssuedAt are standard JWT temporal claims. Extra holds
// any additional application-specific claims.
type Claims struct {
	Subject   string
	Role      string
	ExpiresAt time.Time
	IssuedAt  time.Time
	Extra     map[string]any
}

// Pre-computed base64url-encoded header for HMAC-SHA256:
// {"alg":"HS256","typ":"JWT"}
const jwtHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

// Reserved claim keys that cannot be overridden via Extra.
var reservedClaims = map[string]struct{}{
	"sub": {}, "role": {}, "exp": {}, "iat": {},
}

// SignToken creates a signed JWT (HMAC-SHA256) from the given claims.
// The secret must be non-empty. Returns a compact JWS string
// (header.payload.signature).
func SignToken(secret []byte, c Claims) (string, error) {
	if len(secret) == 0 {
		return "", ErrSecretEmpty
	}

	payload := make(map[string]any, 4+len(c.Extra))
	payload["sub"] = c.Subject
	if c.Role != "" {
		payload["role"] = c.Role
	}
	payload["exp"] = c.ExpiresAt.Unix()
	payload["iat"] = c.IssuedAt.Unix()

	for k, v := range c.Extra {
		if _, reserved := reservedClaims[k]; !reserved {
			payload[k] = v
		}
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	sigInput := jwtHeader + "." + payloadB64

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return sigInput + "." + sig, nil
}

// VerifyToken parses a compact JWS string and validates its HMAC-SHA256
// signature and expiration. Returns the decoded claims on success.
func VerifyToken(secret []byte, token string) (*Claims, error) {
	if len(secret) == 0 {
		return nil, ErrSecretEmpty
	}

	header, payload, sig, ok := splitToken(token)
	if !ok {
		return nil, ErrTokenMalformed
	}

	// Verify signature.
	sigInput := header + "." + payload
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigInput))
	expected := mac.Sum(nil)

	actual, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return nil, ErrTokenMalformed
	}
	if !hmac.Equal(expected, actual) {
		return nil, ErrTokenInvalid
	}

	// Decode payload.
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, ErrTokenMalformed
	}

	var raw map[string]any
	if err := json.Unmarshal(payloadBytes, &raw); err != nil {
		return nil, ErrTokenMalformed
	}

	claims := &Claims{}
	if v, ok := raw["sub"].(string); ok {
		claims.Subject = v
	}
	if v, ok := raw["role"].(string); ok {
		claims.Role = v
	}
	if v, ok := raw["exp"].(float64); ok {
		claims.ExpiresAt = time.Unix(int64(v), 0)
	}
	if v, ok := raw["iat"].(float64); ok {
		claims.IssuedAt = time.Unix(int64(v), 0)
	}

	// Check expiration.
	if !claims.ExpiresAt.IsZero() && time.Now().After(claims.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Collect extra claims.
	for k, v := range raw {
		if _, reserved := reservedClaims[k]; reserved {
			continue
		}
		if claims.Extra == nil {
			claims.Extra = make(map[string]any)
		}
		claims.Extra[k] = v
	}

	return claims, nil
}

// ---------------------------------------------------------------------------
// Auth context helpers
// ---------------------------------------------------------------------------

type claimsKey struct{}

// WithClaims returns a copy of ctx with the auth claims stored.
// Used by the [Auth] middleware; rarely called directly by handlers.
func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

// ClaimsFromCtx extracts auth claims from ctx, or returns nil if no
// claims are present. Use after the [Auth] middleware has run.
func ClaimsFromCtx(ctx context.Context) *Claims {
	if c, ok := ctx.Value(claimsKey{}).(*Claims); ok {
		return c
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

// splitToken splits a JWT compact serialization into its three parts.
// Returns false if the token doesn't contain exactly two dots.
func splitToken(s string) (header, payload, sig string, ok bool) {
	dot1 := strings.IndexByte(s, '.')
	if dot1 < 0 {
		return "", "", "", false
	}
	rest := s[dot1+1:]
	dot2 := strings.IndexByte(rest, '.')
	if dot2 < 0 {
		return "", "", "", false
	}
	// Reject tokens with more than two dots.
	if strings.IndexByte(rest[dot2+1:], '.') >= 0 {
		return "", "", "", false
	}
	return s[:dot1], rest[:dot2], rest[dot2+1:], true
}
