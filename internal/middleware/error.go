package middleware

import (
	"errors"

	"github.com/gofiber/fiber/v2"

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

	fiberutil.CaptureServerError(c, err)

	return baseErrorHandler(c, err)
}
