package middleware

import (
	"errors"

	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ErrorHandler is the global error handler for Fiber.
// It converts fiber.Error and ValidationError types to proper HTTP JSON responses.
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal server error"

	if err != nil {
		// Check for validation errors first
		var validationErr *fiberutil.ValidationError
		if errors.As(err, &validationErr) {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"success": false,
				"message": "Validation failed",
				"errors":  validationErr.Errors,
			})
		}

		// Check for Fiber's built-in error type
		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			code = fiberErr.Code
			message = fiberErr.Message
		} else {
			message = err.Error()
		}

		// Capture 5xx errors to Sentry (if Sentry is enabled)
		if code >= 500 {
			if hub := sentryfiber.GetHubFromContext(c); hub != nil {
				hub.CaptureException(err)
			}
		}
	}

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"message": message,
	})
}
