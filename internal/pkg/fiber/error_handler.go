package fiber

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

// ErrorResponse is the standard error response format
type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ValidationErrorResponse is the response format for validation errors
type ValidationErrorResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Errors  map[string][]string `json:"errors"`
}

// NewErrorHandler returns a Fiber error handler that formats JSON responses
func NewErrorHandler() fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		// Check for validation errors first
		var validationErr *ValidationError
		if errors.As(err, &validationErr) {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(ValidationErrorResponse{
				Success: false,
				Message: "Validation failed",
				Errors:  validationErr.Errors,
			})
		}

		code := fiber.StatusInternalServerError
		message := MsgInternal

		var e *fiber.Error
		if errors.As(err, &e) {
			code = e.Code
			message = e.Message
		}

		return c.Status(code).JSON(ErrorResponse{
			Success: false,
			Message: message,
		})
	}
}
