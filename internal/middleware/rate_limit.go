package middleware

import (
	"fmt"
	"strconv"
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

// RateLimit provides IP-based rate limiting using the globally initialized cache.
// Returns a no-op if the cache has not been initialized.
func RateLimit(maxRequests int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if rateLimitMiddleware.cache == nil {
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
		return rateLimitRequest(c, cfg.Cache, cfg.Max, cfg.Window, keyFunc(c))
	}
}

// GlobalRateLimit applies a broad rate limit to all requests by IP.
// Intended for use as global middleware to throttle scanners and bots.
func GlobalRateLimit(maxRequests int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if rateLimitMiddleware.cache == nil {
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

// rateLimitByKey is the core rate limiting logic
func rateLimitByKey(c *fiber.Ctx, store cache.Cache, maxRequests int, window time.Duration, key string) error {
	val, err := store.Get(c.Context(), key)
	if err != nil {
		// Key doesn't exist yet — first request in window
		if setErr := store.Set(c.Context(), key, "1", window); setErr != nil {
			return c.Next()
		}
		return c.Next()
	}

	count, _ := strconv.Atoi(val)
	if count >= maxRequests {
		c.Set("Retry-After", strconv.Itoa(int(window.Seconds())))
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"message": "Too many requests. Please try again later.",
		})
	}

	_ = store.Set(c.Context(), key, strconv.Itoa(count+1), window)

	return c.Next()
}
