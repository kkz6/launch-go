package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// RateLimit provides rate limiting based on IP or user
func RateLimit(maxRequests int, windowSeconds int) fiber.Handler {
	// TODO: Implement proper rate limiting with Redis
	return func(c *fiber.Ctx) error {
		return c.Next()
	}
}
