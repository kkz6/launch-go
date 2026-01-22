package fiber

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/dto"
)

// Success sends a successful JSON response with the given status code.
func Success(c *fiber.Ctx, status int, message string, data any) error {
	return c.Status(status).JSON(dto.NewSuccessResponse(message, data))
}

// SuccessWithMeta sends a successful JSON response with pagination metadata.
func SuccessWithMeta(c *fiber.Ctx, status int, message string, data any, meta dto.PaginationMeta) error {
	return c.Status(status).JSON(dto.NewSuccessResponseWithMeta(message, data, meta))
}

// Error sends an error JSON response with the given status code.
func Error(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(dto.APIResponse{
		Success: false,
		Message: message,
	})
}

// RespondValidationError sends a 422 response with field validation errors.
func RespondValidationError(c *fiber.Ctx, errs map[string][]string) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
		"success": false,
		"message": MsgValidation,
		"errors":  errs,
	})
}

// OK sends a 200 response with data.
func OK(c *fiber.Ctx, message string, data any) error {
	return Success(c, fiber.StatusOK, message, data)
}

// Created sends a 201 response with data.
func Created(c *fiber.Ctx, message string, data any) error {
	return Success(c, fiber.StatusCreated, message, data)
}

// NoContent sends a 204 response with no body.
func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// RespondBadRequest sends a 400 response.
func RespondBadRequest(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusBadRequest, message)
}

// RespondUnauthorized sends a 401 response.
func RespondUnauthorized(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusUnauthorized, message)
}

// RespondForbidden sends a 403 response.
func RespondForbidden(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusForbidden, message)
}

// RespondNotFound sends a 404 response.
func RespondNotFound(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusNotFound, message)
}

// RespondConflict sends a 409 response.
func RespondConflict(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusConflict, message)
}

// RespondInternalError sends a 500 response.
func RespondInternalError(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusInternalServerError, message)
}

// HandleError checks if error is a fiber.Error and returns appropriate response.
// Use this when you want to handle the response immediately in the handler
// instead of letting the global error handler handle it.
func HandleError(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	code := fiber.StatusInternalServerError
	message := MsgInternal

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		message = e.Message
	}

	return Error(c, code, message)
}

// HandleErrorOrInternal handles fiber.Error or returns internal server error.
// Use this when you want to hide internal error details from the client.
func HandleErrorOrInternal(c *fiber.Ctx, err error, message string) error {
	if err == nil {
		return nil
	}

	var e *fiber.Error
	if errors.As(err, &e) {
		return Error(c, e.Code, e.Message)
	}

	return RespondInternalError(c, message)
}

// MustHandle handles the error or returns success response.
func MustHandle(c *fiber.Ctx, err error, successMsg string, data any) error {
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
