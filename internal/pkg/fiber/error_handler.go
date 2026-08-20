package fiber

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/apperror"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

// ErrorResponse is the standard error response format. Code is optional and
// is populated when an AppError flows through; existing 2xx-returning DTOs
// are unaffected because the field is omitempty.
type ErrorResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// ValidationErrorResponse is the response format for validation errors
type ValidationErrorResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Errors  map[string][]string `json:"errors"`
}

// NewErrorHandler returns a Fiber error handler that formats JSON responses.
//
// This is the canonical, transport-agnostic error → JSON translation. The
// global app handler in internal/middleware/error.go wraps this with
// Sentry capture for 5xx errors; tests should use this directly so they
// exercise the same translation logic as production minus the side-effect
// observability.
//
// Recognised error shapes (in priority order):
//
//  1. *ValidationError — rendered as 422 with the per-field errors map.
//  2. *apperror.AppError — uses HTTPStatus + Code + Message.
//  3. *fiber.Error — uses Code + Message (legacy).
//  4. anything else — 500 with a generic message; the underlying err is
//     returned to whoever wraps this so they can log/forward it.
func NewErrorHandler() fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		// Validation errors (field-level) — render as 422 with the errors
		// map.
		var validationErr *ValidationError
		if errors.As(err, &validationErr) {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(ValidationErrorResponse{
				Success: false,
				Message: i18n.T(c, MsgValidation),
				Errors:  validationErr.LocalizedErrors(c),
			})
		}

		// AppError (preferred path for new handlers).
		if appErr := apperror.As(err); appErr != nil {
			return c.Status(appErr.HTTPStatus).JSON(ErrorResponse{
				Success: false,
				Code:    appErr.Code,
				Message: i18n.T(c, appErr.Message),
			})
		}

		// Fiber's built-in error type (legacy path).
		code := fiber.StatusInternalServerError
		message := MsgInternal
		var e *fiber.Error
		if errors.As(err, &e) {
			code = e.Code
			message = e.Message
		}

		return c.Status(code).JSON(ErrorResponse{
			Success: false,
			Message: i18n.T(c, message),
		})
	}
}
