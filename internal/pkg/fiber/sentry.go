package fiber

import (
	"errors"

	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/apperror"
)

func CaptureServerError(c *fiber.Ctx, err error) {
	if !isServerError(err) {
		return
	}

	hub := sentryfiber.GetHubFromContext(c)
	if hub != nil {
		hub.CaptureException(err)
	}
}

func isServerError(err error) bool {
	if err == nil {
		return false
	}

	if appErr := apperror.As(err); appErr != nil {
		return appErr.HTTPStatus >= fiber.StatusInternalServerError
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return fiberErr.Code >= fiber.StatusInternalServerError
	}

	return true
}
