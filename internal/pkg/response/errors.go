package response

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// AppError represents an application error with HTTP status and message
type AppError struct {
	Err     error
	Status  int
	Message string
}

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

// HandleError checks if error is an AppError and returns appropriate response
// Returns the error response, or nil if it's not an AppError
func HandleError(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	// Check if it's an AppError
	var appErr *AppError
	if errors.As(err, &appErr) {
		return Error(c, appErr.Status, appErr.Message)
	}

	// Default to bad request with error message
	return Error(c, http.StatusBadRequest, err.Error())
}

// HandleErrorOrInternal handles AppError or returns internal server error
func HandleErrorOrInternalErr(c *fiber.Ctx, err error, message string) error {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return Error(c, appErr.Status, appErr.Message)
	}

	return InternalError(c, message)
}
