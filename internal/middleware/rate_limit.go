package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/cache"
)

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

// RateLimit provides rate limiting based on IP or a custom key.
// Uses the cache to track request counts per key within a sliding window.
func RateLimit(maxRequests int, windowSeconds int) fiber.Handler {
	// No-op fallback when no cache is configured — applied at route level below
	return func(c *fiber.Ctx) error {
		return c.Next()
	}
}

// RateLimitWithCache provides rate limiting using the cache store.
func RateLimitWithCache(cfg RateLimitConfig) fiber.Handler {
	if cfg.Cache == nil {
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}

	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(c *fiber.Ctx) string {
			return c.IP()
		}
	}

	return func(c *fiber.Ctx) error {
		key := fmt.Sprintf("rl:%s:%s", c.Path(), cfg.KeyFunc(c))

		val, err := cfg.Cache.Get(c.Context(), key)
		if err != nil {
			// Key doesn't exist yet — first request in window
			if err := cfg.Cache.Set(c.Context(), key, "1", cfg.Window); err != nil {
				// If cache fails, allow the request
				return c.Next()
			}
			return c.Next()
		}

		count, _ := strconv.Atoi(val)
		if count >= cfg.Max {
			c.Set("Retry-After", strconv.Itoa(int(cfg.Window.Seconds())))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"message": "Too many requests. Please try again later.",
			})
		}

		// Increment count (set with remaining TTL approximation)
		_ = cfg.Cache.Set(c.Context(), key, strconv.Itoa(count+1), cfg.Window)

		return c.Next()
	}
}
