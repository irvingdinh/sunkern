package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Password hashing errors.
var (
	// ErrPasswordTooLong is returned when the password exceeds 72 bytes.
	// This matches bcrypt's limit and prevents DoS via extremely long
	// passwords that would take proportionally longer to hash.
	ErrPasswordTooLong = errors.New("password: exceeds 72 byte limit")

	// ErrHashMalformed is returned when the stored hash string cannot be
	// parsed. This typically means the hash was corrupted or was not
	// produced by [HashPassword].
	ErrHashMalformed = errors.New("password: malformed hash")

	// ErrPasswordMismatch is returned by [CheckPassword] when the
	// provided password does not match the stored hash.
	ErrPasswordMismatch = errors.New("password: mismatch")
)

// Default PBKDF2 parameters. OWASP recommends 600,000 iterations for
// PBKDF2-HMAC-SHA256 as of 2023. Salt is 16 bytes, derived key is 32
// bytes (SHA-256 output size).
const (
	defaultIterations = 600_000
	saltLen           = 16
	keyLen            = 32
)

// PasswordOption configures [HashPassword].
type PasswordOption func(*passwordConfig)

type passwordConfig struct {
	iterations int
}

// WithIterations overrides the default PBKDF2 iteration count.
// Use this for testing (lower) or extra security (higher). The default
// is 600,000 per OWASP recommendations.
func WithIterations(n int) PasswordOption {
	return func(c *passwordConfig) {
		if n > 0 {
			c.iterations = n
		}
	}
}

// HashPassword hashes a plaintext password using PBKDF2-HMAC-SHA256.
// Returns a self-describing string in PHC format:
//
//	$pbkdf2-sha256$600000$<base64-salt>$<base64-hash>
//
// The hash encodes the algorithm, iteration count, salt, and derived key
// so that [CheckPassword] can verify without external configuration.
//
//	hash, err := middleware.HashPassword("user-password")
//	// store hash in database
func HashPassword(password string, opts ...PasswordOption) (string, error) {
	if len(password) > 72 {
		return "", ErrPasswordTooLong
	}

	cfg := &passwordConfig{iterations: defaultIterations}
	for _, opt := range opts {
		opt(cfg)
	}

	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password: generating salt: %w", err)
	}

	dk := pbkdf2([]byte(password), salt, cfg.iterations, keyLen)

	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(dk)

	return fmt.Sprintf("$pbkdf2-sha256$%d$%s$%s", cfg.iterations, saltB64, hashB64), nil
}

// CheckPassword compares a plaintext password against a hash produced by
// [HashPassword]. Returns nil on match, [ErrPasswordMismatch] on mismatch,
// or [ErrHashMalformed] if the hash string is invalid.
//
//	if err := middleware.CheckPassword(storedHash, "user-input"); err != nil {
//	    // authentication failed
//	}
func CheckPassword(hash, password string) error {
	if len(password) > 72 {
		return ErrPasswordMismatch
	}

	iterations, salt, expected, err := parseHash(hash)
	if err != nil {
		return err
	}

	actual := pbkdf2([]byte(password), salt, iterations, len(expected))

	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

// ---------------------------------------------------------------------------
// PBKDF2-HMAC-SHA256 (RFC 2898 / NIST SP 800-132)
// ---------------------------------------------------------------------------

// pbkdf2 derives a key from password and salt using PBKDF2 with
// HMAC-SHA256 as the PRF. Implemented per RFC 2898 Section 5.2.
func pbkdf2(password, salt []byte, iter, dkLen int) []byte {
	numBlocks := (dkLen + sha256.Size - 1) / sha256.Size
	dk := make([]byte, 0, numBlocks*sha256.Size)

	blockIdx := make([]byte, 4)
	for i := 1; i <= numBlocks; i++ {
		blockIdx[0] = byte(i >> 24)
		blockIdx[1] = byte(i >> 16)
		blockIdx[2] = byte(i >> 8)
		blockIdx[3] = byte(i)

		// U_1 = PRF(password, salt || INT(i))
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write(blockIdx)
		u := mac.Sum(nil)

		// T_i = U_1 ^ U_2 ^ ... ^ U_c
		t := make([]byte, len(u))
		copy(t, u)

		for j := 1; j < iter; j++ {
			mac.Reset()
			mac.Write(u)
			u = mac.Sum(u[:0])
			for k := range t {
				t[k] ^= u[k]
			}
		}

		dk = append(dk, t...)
	}

	return dk[:dkLen]
}

// ---------------------------------------------------------------------------
// Hash string parsing
// ---------------------------------------------------------------------------

// parseHash extracts iterations, salt, and hash from a PHC-format string.
// Expected format: $pbkdf2-sha256$<iterations>$<base64-salt>$<base64-hash>
func parseHash(s string) (iterations int, salt, hash []byte, err error) {
	// Strip leading $ and split by $.
	if len(s) < 1 || s[0] != '$' {
		return 0, nil, nil, ErrHashMalformed
	}

	parts := strings.Split(s[1:], "$")
	if len(parts) != 4 {
		return 0, nil, nil, ErrHashMalformed
	}

	if parts[0] != "pbkdf2-sha256" {
		return 0, nil, nil, ErrHashMalformed
	}

	iterations, convErr := strconv.Atoi(parts[1])
	if convErr != nil || iterations <= 0 {
		return 0, nil, nil, ErrHashMalformed
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) == 0 {
		return 0, nil, nil, ErrHashMalformed
	}

	hash, err = base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(hash) == 0 {
		return 0, nil, nil, ErrHashMalformed
	}

	return iterations, salt, hash, nil
}
