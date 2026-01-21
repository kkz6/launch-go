package response

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"

	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// AppError is an alias to the canonical AppError type in pkg/errors.
// Use apperrors.AppError directly for new code.
type AppError = apperrors.AppError

// ResourceError is an alias to the canonical ResourceError type in pkg/errors.
type ResourceError = apperrors.ResourceError

// NewAppError creates a new ResourceError for backwards compatibility.
// For new code, use apperrors.NewAppError which supports error codes.
func NewAppError(status int, message string) *ResourceError {
	return apperrors.WithStatus(status, message)
}

// Common error constructors - delegate to apperrors package
func ErrNotFound(message string) *ResourceError {
	return apperrors.NotFound(message)
}

func ErrBadRequest(message string) *ResourceError {
	return apperrors.BadRequest(message)
}

func ErrConflict(message string) *ResourceError {
	return apperrors.Conflict(message)
}

func ErrForbidden(message string) *ResourceError {
	return apperrors.Forbidden(message)
}

func ErrUnauthorized(message string) *ResourceError {
	return apperrors.Unauthorized(message)
}

func ErrInternal(message string) *ResourceError {
	return apperrors.Internal(message)
}

// HTTPStatusError is an alias to the canonical interface in pkg/errors.
// This is implemented by:
// - AppError (pkg/errors)
// - ResourceError (pkg/errors)
// - ModelError (pkg/repository)
type HTTPStatusError = apperrors.HTTPStatusError

// =============================================================================
// Laravel-style Abort Functions
// =============================================================================
// These functions allow handlers to simply return the error and let
// the error middleware handle the HTTP response automatically.
//
// Usage in handlers:
//   return response.Abort(apperrors.ErrServerNotFound)
//   return response.AbortNotFound("Server")
//   return response.AbortBadRequest("Invalid input")
// =============================================================================

// Abort returns the error directly - the error middleware will convert it
// to the appropriate HTTP response based on the error's HTTPStatus() method.
// This is similar to Laravel's abort() helper.
//
// Example:
//
//	site, err := h.service.GetSite(ctx, id)
//	if err != nil {
//	    return response.Abort(err)  // Automatically returns 404 for ErrSiteNotFound
//	}
func Abort(err error) error {
	return err
}

// AbortNotFound creates and returns a 404 Not Found error
// Example: return response.AbortNotFound("Server")  // "Server not found"
func AbortNotFound(resource string) error {
	return NewAppError(http.StatusNotFound, resource+" not found")
}

// AbortBadRequest creates and returns a 400 Bad Request error
// Example: return response.AbortBadRequest("Invalid server ID")
func AbortBadRequest(message string) error {
	return NewAppError(http.StatusBadRequest, message)
}

// AbortForbidden creates and returns a 403 Forbidden error
// Example: return response.AbortForbidden("You don't have permission")
func AbortForbidden(message string) error {
	return NewAppError(http.StatusForbidden, message)
}

// AbortUnauthorized creates and returns a 401 Unauthorized error
// Example: return response.AbortUnauthorized("Please login")
func AbortUnauthorized(message string) error {
	return NewAppError(http.StatusUnauthorized, message)
}

// AbortConflict creates and returns a 409 Conflict error
// Example: return response.AbortConflict("Resource already exists")
func AbortConflict(message string) error {
	return NewAppError(http.StatusConflict, message)
}

// AbortInternal creates and returns a 500 Internal Server Error
// Example: return response.AbortInternal("Something went wrong")
func AbortInternal(message string) error {
	return NewAppError(http.StatusInternalServerError, message)
}

// AbortWithStatus creates an error with a custom status code
// Example: return response.AbortWithStatus(429, "Too many requests")
func AbortWithStatus(status int, message string) error {
	return NewAppError(status, message)
}

// =============================================================================
// Handler Helper Functions (for explicit response control)
// =============================================================================

// HandleError checks if error is an HTTPStatusError and returns appropriate response
// Use this when you want to handle the response immediately in the handler
// instead of letting the middleware handle it.
//
// Returns the error response, or nil if err is nil
func HandleError(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	// Check for HTTPStatusError interface (catches AppError, ResourceError, ModelError)
	var httpErr HTTPStatusError
	if errors.As(err, &httpErr) {
		return Error(c, httpErr.HTTPStatus(), err.Error())
	}

	// Default to bad request with error message
	return Error(c, http.StatusBadRequest, err.Error())
}

// HandleErrorOrInternal handles HTTPStatusError or returns internal server error
// Use this when you want to hide internal error details from the client
func HandleErrorOrInternalErr(c *fiber.Ctx, err error, message string) error {
	if err == nil {
		return nil
	}

	// Check for HTTPStatusError interface
	var httpErr HTTPStatusError
	if errors.As(err, &httpErr) {
		return Error(c, httpErr.HTTPStatus(), err.Error())
	}

	// Return generic internal error message (hides implementation details)
	return InternalError(c, message)
}

// MustHandle handles the error or returns nil for success
// Use with service calls: return response.MustHandle(c, err, data)
func MustHandle(c *fiber.Ctx, err error, successMsg string, data interface{}) error {
	if err != nil {
		return HandleError(c, err)
	}
	return OK(c, successMsg, data)
}
