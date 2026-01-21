package middleware

import (
	"errors"

	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"

	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// ErrorHandler is the global error handler for Fiber
// It automatically converts our custom error types to proper HTTP responses
// Similar to Laravel's exception handler
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal server error"

	if err != nil {
		message = err.Error()

		// Check for Fiber's built-in error type first
		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			code = fiberErr.Code
		} else {
			// Check for our custom HTTPStatusError interface
			// This catches ResourceError, AppError, and ModelError
			var httpErr apperrors.HTTPStatusError
			if errors.As(err, &httpErr) {
				code = httpErr.HTTPStatus()
			}
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
