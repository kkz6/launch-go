package middleware

import "github.com/gofiber/fiber/v2"

// SecurityHeaders adds standard security headers to all responses.
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "0")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		return c.Next()
	}
}
