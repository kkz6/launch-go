package response

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"

	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// AppError represents an application error with HTTP status and message.
// Implements HTTPStatusError from internal/pkg/errors.
type AppError struct {
	Err     error
	Status  int
	Message string
}

// Compile-time check that AppError implements HTTPStatusError
var _ apperrors.HTTPStatusError = (*AppError)(nil)

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

// Unwrap allows errors.Is and errors.As to work
func (e *AppError) Unwrap() error {
	return e.Err
}

// HTTPStatus implements HTTPStatusError interface
func (e *AppError) HTTPStatus() int {
	return e.Status
}

// NewAppError creates a new AppError
func NewAppError(status int, message string) *AppError {
	return &AppError{
		Err:     errors.New(message),
		Status:  status,
		Message: message,
	}
}

// Common error constructors
func ErrNotFound(message string) *AppError {
	return NewAppError(http.StatusNotFound, message)
}

func ErrBadRequest(message string) *AppError {
	return NewAppError(http.StatusBadRequest, message)
}

func ErrConflict(message string) *AppError {
	return NewAppError(http.StatusConflict, message)
}

func ErrForbidden(message string) *AppError {
	return NewAppError(http.StatusForbidden, message)
}

func ErrUnauthorized(message string) *AppError {
	return NewAppError(http.StatusUnauthorized, message)
}

func ErrInternal(message string) *AppError {
	return NewAppError(http.StatusInternalServerError, message)
}

// HTTPStatusError is an alias to the canonical interface in pkg/errors.
// This is implemented by:
// - AppError (this package)
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
