package fiber

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// HandleServiceError converts service errors to appropriate HTTP responses.
// It checks for ResourceError (HTTP-aware), AppError, and common error patterns.
// Returns nil if err is nil.
func HandleServiceError(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	// Check for ResourceError (HTTP-aware error with status code)
	var resourceErr *apperrors.ResourceError
	if errors.As(err, &resourceErr) {
		return response.Error(c, resourceErr.HTTPStatus(), resourceErr.Message)
	}

	// Check for AppError (legacy error type)
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return response.Error(c, appErr.Code, appErr.Message)
	}

	// Check for common error patterns
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return response.NotFound(c, "Resource not found")
	case errors.Is(err, context.Canceled):
		return response.Error(c, fiber.StatusRequestTimeout, "Request cancelled")
	case errors.Is(err, context.DeadlineExceeded):
		return response.Error(c, fiber.StatusGatewayTimeout, "Request timeout")
	case errors.Is(err, apperrors.ErrNotFound):
		return response.NotFound(c, "Resource not found")
	case errors.Is(err, apperrors.ErrUnauthorized):
		return response.Unauthorized(c, "Unauthorized")
	case errors.Is(err, apperrors.ErrForbidden):
		return response.Forbidden(c, "Access denied")
	case errors.Is(err, apperrors.ErrValidation):
		return response.BadRequest(c, err.Error())
	case errors.Is(err, apperrors.ErrBadRequest):
		return response.BadRequest(c, err.Error())
	case errors.Is(err, apperrors.ErrConflict):
		return response.Error(c, fiber.StatusConflict, err.Error())
	default:
		return response.InternalError(c, "An unexpected error occurred")
	}
}

// HandleServiceErrorWithMessage is like HandleServiceError but uses a custom message
// for internal/unexpected errors instead of the default generic message.
func HandleServiceErrorWithMessage(c *fiber.Ctx, err error, internalMsg string) error {
	if err == nil {
		return nil
	}

	// Check for ResourceError (HTTP-aware error with status code)
	var resourceErr *apperrors.ResourceError
	if errors.As(err, &resourceErr) {
		return response.Error(c, resourceErr.HTTPStatus(), resourceErr.Message)
	}

	// Check for AppError (legacy error type)
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return response.Error(c, appErr.Code, appErr.Message)
	}

	// Check for common error patterns
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return response.NotFound(c, "Resource not found")
	case errors.Is(err, context.Canceled):
		return response.Error(c, fiber.StatusRequestTimeout, "Request cancelled")
	case errors.Is(err, context.DeadlineExceeded):
		return response.Error(c, fiber.StatusGatewayTimeout, "Request timeout")
	case errors.Is(err, apperrors.ErrNotFound):
		return response.NotFound(c, "Resource not found")
	case errors.Is(err, apperrors.ErrUnauthorized):
		return response.Unauthorized(c, "Unauthorized")
	case errors.Is(err, apperrors.ErrForbidden):
		return response.Forbidden(c, "Access denied")
	case errors.Is(err, apperrors.ErrValidation):
		return response.BadRequest(c, err.Error())
	case errors.Is(err, apperrors.ErrBadRequest):
		return response.BadRequest(c, err.Error())
	case errors.Is(err, apperrors.ErrConflict):
		return response.Error(c, fiber.StatusConflict, err.Error())
	default:
		return response.InternalError(c, internalMsg)
	}
}

// MustSucceed is a helper that returns nil if err is nil,
// otherwise handles the error and returns the response error.
// Use this for simple "return on error" patterns.
func MustSucceed(c *fiber.Ctx, err error) error {
	return HandleServiceError(c, err)
}
