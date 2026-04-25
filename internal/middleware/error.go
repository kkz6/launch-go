package middleware

import (
	"errors"

	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/apperror"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// baseErrorHandler is the shared error → JSON translation used by both
// production and tests. Owned by internal/pkg/fiber so test helpers that
// build standalone Fiber apps exercise the same behaviour.
var baseErrorHandler = fiberutil.NewErrorHandler()

// ErrorHandler is the global error handler for Fiber.
// It delegates the actual error → JSON translation to fiberutil.NewErrorHandler
// (recognises AppError, fiber.Error, and ValidationError) and adds the only
// production-only concern: forwarding 5xx errors to Sentry.
func ErrorHandler(c *fiber.Ctx, err error) error {
	if err == nil {
		err = errors.New("nil error reached global handler")
	}

	captureIfServerError(c, err)

	return baseErrorHandler(c, err)
}

// captureIfServerError forwards 5xx errors to Sentry when the integration is
// enabled. Determines server-error-ness from the typed error rather than the
// response status — by the time the response is written, we'd have lost the
// chance to capture.
func captureIfServerError(c *fiber.Ctx, err error) {
	hub := sentryfiber.GetHubFromContext(c)
	if hub == nil {
		return
	}

	if appErr := apperror.As(err); appErr != nil {
		if appErr.HTTPStatus >= 500 {
			hub.CaptureException(err)
		}
		return
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		if fiberErr.Code >= 500 {
			hub.CaptureException(err)
		}
		return
	}

	// Plain errors are treated as 500.
	hub.CaptureException(err)
}
