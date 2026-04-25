package middleware

import (
	"errors"

	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/apperror"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ErrorHandler is the global error handler for Fiber.
// It converts AppError, fiber.Error, and ValidationError types to proper HTTP
// JSON responses, and forwards 5xx errors to Sentry when configured.
func ErrorHandler(c *fiber.Ctx, err error) error {
	if err == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error",
		})
	}

	// Validation errors (field-level) — render as 422 with the errors map.
	var validationErr *fiberutil.ValidationError
	if errors.As(err, &validationErr) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "Validation failed",
			"errors":  validationErr.Errors,
		})
	}

	// AppError (preferred path for new handlers) — carries a stable code,
	// HTTP status, and a wrapped underlying error that we log but never
	// expose to the client.
	if appErr := apperror.As(err); appErr != nil {
		captureIfServerError(c, appErr.HTTPStatus, err)
		return c.Status(appErr.HTTPStatus).JSON(fiber.Map{
			"success": false,
			"code":    appErr.Code,
			"message": appErr.Message,
		})
	}

	// Fiber's built-in error type — existing handlers using fiber.NewError
	// continue to work unchanged.
	code := fiber.StatusInternalServerError
	message := err.Error()
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
		message = fiberErr.Message
	}

	captureIfServerError(c, code, err)

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"message": message,
	})
}

// captureIfServerError forwards 5xx errors to Sentry when the integration is
// enabled. Pulled out so the AppError and fiber.Error branches share behaviour.
func captureIfServerError(c *fiber.Ctx, status int, err error) {
	if status < 500 {
		return
	}
	if hub := sentryfiber.GetHubFromContext(c); hub != nil {
		hub.CaptureException(err)
	}
}
