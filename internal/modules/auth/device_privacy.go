package auth

import (
	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"
)

// Device codes and second-factor credentials must not accompany error reports.
// Run before authentication and validation, which can also reject a request.
func scrubDeviceRequest(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if hub := sentryfiber.GetHubFromContext(c); hub != nil {
		hub.Scope().SetRequestBody(nil)
	}
	return c.Next()
}
