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

// NewErrorHandler returns a Fiber error handler that formats JSON responses
func NewErrorHandler() fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
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
