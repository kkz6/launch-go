// Package errors provides a consolidated error handling system for the application.
// It defines the core error types and interfaces used throughout the codebase.
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// HTTPStatusError is an interface for errors that carry HTTP status codes.
// Implemented by AppError and ResourceError.
type HTTPStatusError interface {
	error
	HTTPStatus() int
}

// AppError represents an application error with HTTP status, error code, and message.
// It supports error wrapping and implements the HTTPStatusError interface.
type AppError struct {
	// Cause is the underlying error that caused this error (for wrapping)
	Cause error `json:"-"`
	// Code is a machine-readable error code (e.g., "validation_error", "not_found")
	Code string `json:"code,omitempty"`
	// Message is a human-readable error message
	Message string `json:"message"`
	// StatusCode is the HTTP status code associated with this error
	StatusCode int `json:"status_code"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "unknown error"
}

// Unwrap returns the underlying cause error, enabling errors.Is and errors.As
func (e *AppError) Unwrap() error {
	return e.Cause
}

// HTTPStatus returns the HTTP status code for this error
func (e *AppError) HTTPStatus() int {
	if e.StatusCode != 0 {
		return e.StatusCode
	}
	return http.StatusInternalServerError
}

// Is reports whether the error matches the target error.
// It checks both the Code and the underlying Cause.
func (e *AppError) Is(target error) bool {
	if t, ok := target.(*AppError); ok {
		// If both have codes, compare codes
		if e.Code != "" && t.Code != "" {
			return e.Code == t.Code
		}
	}
	return false
}

// WithCause returns a copy of the error with the given cause
func (e *AppError) WithCause(cause error) *AppError {
	return &AppError{
		Cause:      cause,
		Code:       e.Code,
		Message:    e.Message,
		StatusCode: e.StatusCode,
	}
}

// WithMessage returns a copy of the error with the given message
func (e *AppError) WithMessage(message string) *AppError {
	return &AppError{
		Cause:      e.Cause,
		Code:       e.Code,
		Message:    message,
		StatusCode: e.StatusCode,
	}
}

// NewAppError creates a new AppError with the given parameters
func NewAppError(code string, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// NewAppErrorWithCause creates a new AppError wrapping the given cause
func NewAppErrorWithCause(cause error, code string, message string, statusCode int) *AppError {
	return &AppError{
		Cause:      cause,
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// ResourceError represents an HTTP-aware error for a specific resource.
// It's simpler than AppError and used primarily for not-found and conflict errors.
type ResourceError struct {
	Message string
	Status  int
}

// Error implements the error interface
func (e *ResourceError) Error() string {
	return e.Message
}

// HTTPStatus returns the HTTP status code for this error
func (e *ResourceError) HTTPStatus() int {
	return e.Status
}

// Constructor functions for ResourceError

// NotFound creates a new not found error with the given message
func NotFound(message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  http.StatusNotFound,
	}
}

// BadRequest creates a new bad request error with the given message
func BadRequest(message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  http.StatusBadRequest,
	}
}

// Conflict creates a new conflict error with the given message
func Conflict(message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  http.StatusConflict,
	}
}

// Internal creates a new internal server error with the given message
func Internal(message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  http.StatusInternalServerError,
	}
}

// Forbidden creates a new forbidden error with the given message
func Forbidden(message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  http.StatusForbidden,
	}
}

// Unauthorized creates a new unauthorized error with the given message
func Unauthorized(message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  http.StatusUnauthorized,
	}
}

// WithStatus creates a new ResourceError with the given status and message
func WithStatus(status int, message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  status,
	}
}

// GetHTTPStatus extracts the HTTP status code from an error.
// Returns the status code if the error implements HTTPStatusError,
// otherwise returns the provided default status.
func GetHTTPStatus(err error, defaultStatus int) int {
	var httpErr HTTPStatusError
	if errors.As(err, &httpErr) {
		return httpErr.HTTPStatus()
	}
	return defaultStatus
}

// IsHTTPStatusError checks if an error implements the HTTPStatusError interface
func IsHTTPStatusError(err error) bool {
	var httpErr HTTPStatusError
	return errors.As(err, &httpErr)
}

// GetErrorMessage extracts the error message, preferring the Message field
// from AppError or ResourceError if available
func GetErrorMessage(err error, defaultMessage string) string {
	if err == nil {
		return defaultMessage
	}

	var appErr *AppError
	if errors.As(err, &appErr) && appErr.Message != "" {
		return appErr.Message
	}

	var resErr *ResourceError
	if errors.As(err, &resErr) && resErr.Message != "" {
		return resErr.Message
	}

	if msg := err.Error(); msg != "" {
		return msg
	}

	return defaultMessage
}

// HTTPStatusFromError determines the HTTP status code based on error type.
// Uses the error chain to find an HTTPStatusError or falls back to 500.
func HTTPStatusFromError(err error) int {
	if err == nil {
		return http.StatusOK
	}

	// Check for Fiber error (common in handlers)
	type fiberError interface {
		error
		Code() int
	}
	var fErr fiberError
	if errors.As(err, &fErr) {
		return fErr.Code()
	}

	// Check for our HTTPStatusError interface
	var httpErr HTTPStatusError
	if errors.As(err, &httpErr) {
		return httpErr.HTTPStatus()
	}

	return http.StatusInternalServerError
}

// ToAppError converts any error to an AppError.
// If the error is already an AppError, it returns it directly.
// Otherwise, it wraps the error with the provided code and status.
func ToAppError(err error, code string, statusCode int) *AppError {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	return &AppError{
		Cause:      err,
		Code:       code,
		Message:    err.Error(),
		StatusCode: statusCode,
	}
}

// New creates a simple error with the given message (alias for errors.New)
func New(message string) error {
	return errors.New(message)
}

// Errorf creates a new error with formatted message (alias for fmt.Errorf)
func Errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
