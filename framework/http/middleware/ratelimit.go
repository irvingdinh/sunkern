package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimitOption configures the RateLimit middleware.
type RateLimitOption func(*rateLimitConfig)

type rateLimitConfig struct {
	keyFunc         func(r *http.Request) string
	cleanupInterval time.Duration
}

// WithKeyFunc sets a custom function to extract the rate limit key from
// each request. The default extracts the client IP from X-Real-IP,
// X-Forwarded-For, or RemoteAddr (in that order).
func WithKeyFunc(fn func(r *http.Request) string) RateLimitOption {
	return func(c *rateLimitConfig) { c.keyFunc = fn }
}

// WithCleanupInterval sets how often stale token buckets are removed from
// memory. Cleanup runs lazily during request processing. Default: 1 minute.
func WithCleanupInterval(d time.Duration) RateLimitOption {
	return func(c *rateLimitConfig) { c.cleanupInterval = d }
}

// RateLimit returns a middleware that limits requests using a token bucket
// algorithm. Each unique key (default: client IP) gets its own bucket that
// refills at rps tokens per second, up to a maximum of burst tokens.
//
// When the bucket is empty, the middleware returns 429 Too Many Requests
// with a Retry-After header.
//
//	api := server.Group("/api")
//	api.Use(middleware.RateLimit(10, 20)) // 10 req/s sustained, burst of 20
func RateLimit(rps float64, burst int, opts ...RateLimitOption) func(http.Handler) http.Handler {
	cfg := &rateLimitConfig{
		keyFunc:         clientIP,
		cleanupInterval: time.Minute,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	rl := &rateLimiter{
		buckets:         make(map[string]*tokenBucket),
		rps:             rps,
		burst:           float64(burst),
		keyFunc:         cfg.keyFunc,
		cleanupInterval: cfg.cleanupInterval,
		lastCleanup:     time.Now(),
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rl.keyFunc(r)
			if !rl.allow(key) {
				w.Header().Set("Retry-After", "1")
				writeErrorJSON(w, http.StatusTooManyRequests, "too_many_requests", "Too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ---------------------------------------------------------------------------
// Token bucket internals
// ---------------------------------------------------------------------------

type tokenBucket struct {
	tokens   float64
	lastTime time.Time
}

type rateLimiter struct {
	mu              sync.Mutex
	buckets         map[string]*tokenBucket
	rps             float64
	burst           float64
	keyFunc         func(r *http.Request) string
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

// allow checks whether a request with the given key is permitted. It refills
// tokens based on elapsed time and tries to consume one token.
func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Lazy cleanup of stale buckets.
	if now.Sub(rl.lastCleanup) >= rl.cleanupInterval {
		rl.cleanup(now)
		rl.lastCleanup = now
	}

	b, ok := rl.buckets[key]
	if !ok {
		// First request from this key — start with a full bucket.
		rl.buckets[key] = &tokenBucket{tokens: rl.burst - 1, lastTime: now}
		return true
	}

	// Refill tokens based on elapsed time.
	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * rl.rps
	if b.tokens > rl.burst {
		b.tokens = rl.burst
	}
	b.lastTime = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// cleanup removes buckets that have been idle long enough to be fully
// refilled. A fully-refilled bucket is indistinguishable from a new one,
// so tracking it wastes memory.
func (rl *rateLimiter) cleanup(now time.Time) {
	refillSeconds := rl.burst / rl.rps
	cutoff := now.Add(-time.Duration(refillSeconds * float64(time.Second)))
	for key, b := range rl.buckets {
		if b.lastTime.Before(cutoff) {
			delete(rl.buckets, key)
		}
	}
}

// ---------------------------------------------------------------------------
// Default key extraction
// ---------------------------------------------------------------------------

// clientIP extracts the client IP address from the request. It checks
// X-Real-IP and X-Forwarded-For headers (common in reverse proxy setups
// like Fly.io, Railway, Cloud Run) before falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	// X-Real-IP is set by most reverse proxies to the actual client IP.
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// X-Forwarded-For is a comma-separated chain. The leftmost IP is
	// typically the original client. In trusted proxy environments (Fly.io,
	// Railway, Cloud Run), the proxy overwrites this header.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Direct connection — strip the port from RemoteAddr.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
