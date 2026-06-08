package middleware

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/cache"
)

// rateLimitMiddleware holds the shared cache for rate limiting
var rateLimitMiddleware struct {
	cache cache.Cache
}

// InitRateLimitMiddleware initializes the rate limit middleware with a cache store.
// Call this during application bootstrap before routes are registered.
func InitRateLimitMiddleware(c cache.Cache) {
	rateLimitMiddleware.cache = c
}

// RateLimitConfig holds configuration for the rate limiter
type RateLimitConfig struct {
	// Max number of requests within the window
	Max int
	// Window duration
	Window time.Duration
	// KeyFunc generates the rate limit key from the request (default: IP-based)
	KeyFunc func(c *fiber.Ctx) string
	// Cache store for rate limit counters
	Cache cache.Cache
}

// rateLimitExemptPaths are never rate limited: liveness/health probes, the
// WebSocket upgrade, and metrics. Health probes hit the API every few seconds
// from the container/orchestrator; counting them lets the bucket fill and 429
// the probe, which flaps the container unhealthy.
var rateLimitExemptPaths = map[string]struct{}{
	"/health":  {},
	"/up":      {},
	"/ws":      {},
	"/metrics": {},
}

// isRateLimitExempt reports whether a request should bypass rate limiting:
// health/liveness/internal paths, and loopback traffic (the in-container health
// probe hits 127.0.0.1 and must never be throttled).
func isRateLimitExempt(c *fiber.Ctx) bool {
	if _, ok := rateLimitExemptPaths[c.Path()]; ok {
		return true
	}
	ip := c.IP()
	return ip == "127.0.0.1" || ip == "::1"
}

// RateLimit provides IP-based rate limiting using the globally initialized cache.
// Returns a no-op if the cache has not been initialized.
func RateLimit(maxRequests int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if rateLimitMiddleware.cache == nil || isRateLimitExempt(c) {
			return c.Next()
		}

		return rateLimitRequest(c, rateLimitMiddleware.cache, maxRequests, window, c.IP())
	}
}

// RateLimitWithCache provides rate limiting using an explicitly provided cache store.
func RateLimitWithCache(cfg RateLimitConfig) fiber.Handler {
	if cfg.Cache == nil {
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}

	keyFunc := cfg.KeyFunc
	if keyFunc == nil {
		keyFunc = func(c *fiber.Ctx) string {
			return c.IP()
		}
	}

	return func(c *fiber.Ctx) error {
		if isRateLimitExempt(c) {
			return c.Next()
		}
		return rateLimitRequest(c, cfg.Cache, cfg.Max, cfg.Window, keyFunc(c))
	}
}

// GlobalRateLimit applies a broad rate limit to all requests by IP.
// Intended for use as global middleware to throttle scanners and bots.
func GlobalRateLimit(maxRequests int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if rateLimitMiddleware.cache == nil || isRateLimitExempt(c) {
			return c.Next()
		}

		key := fmt.Sprintf("rl:global:%s", c.IP())

		return rateLimitByKey(c, rateLimitMiddleware.cache, maxRequests, window, key)
	}
}

// rateLimitRequest checks rate limits using path + identifier as key
func rateLimitRequest(c *fiber.Ctx, store cache.Cache, maxRequests int, window time.Duration, identifier string) error {
	key := fmt.Sprintf("rl:%s:%s", c.Path(), identifier)

	return rateLimitByKey(c, store, maxRequests, window, key)
}

// rateLimitByKey is the core fixed-window rate limiting logic.
//
// The counter value encodes both the count and the window's fixed expiry
// ("count|expiryUnixMillis"). On each increment the TTL is set to the REMAINING
// time until that fixed expiry — never refreshed to a full window — so a key
// that is hit continuously still resets when its window elapses. (The previous
// implementation re-set the TTL to a full window on every request, so a key hit
// faster than the window never expired and stayed at 429 forever.)
func rateLimitByKey(c *fiber.Ctx, store cache.Cache, maxRequests int, window time.Duration, key string) error {
	ctx := c.Context()
	now := time.Now()

	val, err := store.Get(ctx, key)
	if err != nil || val == "" {
		// First request in a new window.
		_ = store.Set(ctx, key, encodeRateLimit(1, now.Add(window)), window)
		return c.Next()
	}

	count, expiry, ok := decodeRateLimit(val)
	if !ok || !now.Before(expiry) {
		// Legacy/garbled value, or the window has elapsed — start fresh.
		_ = store.Set(ctx, key, encodeRateLimit(1, now.Add(window)), window)
		return c.Next()
	}

	if count >= maxRequests {
		retryAfter := int(time.Until(expiry).Seconds()) + 1
		c.Set("Retry-After", strconv.Itoa(retryAfter))
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"message": "Too many requests. Please try again later.",
		})
	}

	// Increment but keep the SAME expiry: TTL = remaining time in the window.
	remaining := time.Until(expiry)
	if remaining <= 0 {
		remaining = window
	}
	_ = store.Set(ctx, key, encodeRateLimit(count+1, expiry), remaining)

	return c.Next()
}

// encodeRateLimit serialises the counter as "count|expiryUnixMillis".
func encodeRateLimit(count int, expiry time.Time) string {
	return fmt.Sprintf("%d|%d", count, expiry.UnixMilli())
}

// decodeRateLimit parses a "count|expiryUnixMillis" value. ok is false for any
// value not in that shape (e.g. a legacy bare integer), so callers treat it as
// the start of a new window.
func decodeRateLimit(s string) (count int, expiry time.Time, ok bool) {
	left, right, found := strings.Cut(s, "|")
	if !found {
		return 0, time.Time{}, false
	}
	count, err := strconv.Atoi(left)
	if err != nil {
		return 0, time.Time{}, false
	}
	ms, err := strconv.ParseInt(right, 10, 64)
	if err != nil {
		return 0, time.Time{}, false
	}
	return count, time.UnixMilli(ms), true
}
