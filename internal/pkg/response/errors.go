package response

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

// HandleError checks if error is a fiber.Error and returns appropriate response.
// Use this when you want to handle the response immediately in the handler
// instead of letting the global error handler handle it.
func HandleError(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	code := fiber.StatusInternalServerError
	message := "Internal server error"

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		message = e.Message
	}

	return Error(c, code, message)
}

// HandleErrorOrInternalErr handles fiber.Error or returns internal server error.
// Use this when you want to hide internal error details from the client.
func HandleErrorOrInternalErr(c *fiber.Ctx, err error, message string) error {
	if err == nil {
		return nil
	}

	var e *fiber.Error
	if errors.As(err, &e) {
		return Error(c, e.Code, e.Message)
	}

	return InternalError(c, message)
}

// MustHandle handles the error or returns success response.
func MustHandle(c *fiber.Ctx, err error, successMsg string, data interface{}) error {
	if err != nil {
		return HandleError(c, err)
	}
	return OK(c, successMsg, data)
}

// Abort returns the error directly for the global error handler to process.
// This is similar to Laravel's abort() helper.
func Abort(err error) error {
	return err
}
