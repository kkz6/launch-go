// Package apperror provides a structured error type that handlers can return
// to give clients a stable error code (independent of HTTP status) plus a
// human-readable message, while preserving the underlying error for logging.
//
// The pattern is borrowed from the goshop project. Compared to wrapping with
// fiber.NewError (which only carries Code + Message), AppError adds:
//
//   - A stable string Code (e.g. "auth.email_taken") that frontends can
//     branch on without parsing prose. HTTP status is a separate concern.
//   - A wrapped Err the global error handler can log to Sentry without
//     leaking to the client.
//   - errors.Is / errors.As compatibility with predefined sentinel values
//     so business logic can pattern-match without string comparisons.
//
// Migration is opt-in: handlers can keep returning fiber.NewError(...) until
// they're touched. The middleware error handler recognizes both shapes.
//
// Usage:
//
//	if existing != nil {
//	    return apperror.ErrConflict.WithMessage("email already registered").
//	        WithCode("auth.email_taken").
//	        Wrap(err)
//	}
package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError is a structured application error. Construct via the predefined
// sentinels (ErrBadRequest, ErrConflict, ...) and the WithX builder methods
// rather than building one from scratch — that keeps the catalog of stable
// codes auditable in one file.
type AppError struct {
	// Code is a stable, machine-readable identifier (e.g. "auth.email_taken").
	// Independent of HTTPStatus so multiple codes can share a status.
	Code string

	// Message is the human-readable text returned to the client.
	Message string

	// HTTPStatus is the HTTP status the global error handler should write.
	HTTPStatus int

	// Err is the underlying cause, preserved for logging / Sentry.
	// It is NEVER serialized to the client.
	Err error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error, enabling errors.Is / errors.As.
func (e *AppError) Unwrap() error {
	return e.Err
}

// Is reports whether target is an AppError with the same Code. Two AppErrors
// with the same Code are considered equal regardless of message or wrapped
// error, so callers can write `errors.Is(err, apperror.ErrNotFound)`.
func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// clone returns a shallow copy. The builder methods all clone first so that
// modifying a derived error never mutates a predefined sentinel.
func (e *AppError) clone() *AppError {
	c := *e
	return &c
}

// WithMessage returns a copy of the error with the given message.
func (e *AppError) WithMessage(msg string) *AppError {
	c := e.clone()
	c.Message = msg
	return c
}

// WithMessagef returns a copy with a formatted message.
func (e *AppError) WithMessagef(format string, args ...any) *AppError {
	c := e.clone()
	c.Message = fmt.Sprintf(format, args...)
	return c
}

// WithCode returns a copy with the given code (typically a more specific
// domain code like "auth.email_taken" replacing the generic "conflict").
func (e *AppError) WithCode(code string) *AppError {
	c := e.clone()
	c.Code = code
	return c
}

// Wrap returns a copy with the underlying error attached. The wrapped error
// is preserved through Unwrap and is what gets logged to Sentry; it is not
// returned to the client.
func (e *AppError) Wrap(err error) *AppError {
	c := e.clone()
	c.Err = err
	return c
}

// As extracts an *AppError from err's chain, returning nil if none is found.
// Convenience wrapper around errors.As for use in middleware/handlers.
func As(err error) *AppError {
	if err == nil {
		return nil
	}
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	return nil
}

// Predefined error codes. These match the HTTP semantics of the corresponding
// status codes, but handlers should derive more specific codes via WithCode
// when the failure has a meaningful domain identity.
const (
	CodeBadRequest         = "bad_request"
	CodeUnauthorized       = "unauthorized"
	CodeForbidden          = "forbidden"
	CodeNotFound           = "not_found"
	CodeConflict           = "conflict"
	CodeValidation         = "validation_failed"
	CodeTooManyRequests    = "too_many_requests"
	CodeInternal           = "internal_error"
	CodeServiceUnavailable = "service_unavailable"
)

// Predefined errors. Treat these as immutable — always derive copies via the
// WithX builders rather than mutating the sentinel directly.
var (
	ErrBadRequest = &AppError{
		Code:       CodeBadRequest,
		Message:    "Bad request",
		HTTPStatus: http.StatusBadRequest,
	}
	ErrUnauthorized = &AppError{
		Code:       CodeUnauthorized,
		Message:    "Unauthorized",
		HTTPStatus: http.StatusUnauthorized,
	}
	ErrForbidden = &AppError{
		Code:       CodeForbidden,
		Message:    "Access denied",
		HTTPStatus: http.StatusForbidden,
	}
	ErrNotFound = &AppError{
		Code:       CodeNotFound,
		Message:    "Resource not found",
		HTTPStatus: http.StatusNotFound,
	}
	ErrConflict = &AppError{
		Code:       CodeConflict,
		Message:    "Resource conflict",
		HTTPStatus: http.StatusConflict,
	}
	ErrValidation = &AppError{
		Code:       CodeValidation,
		Message:    "Validation failed",
		HTTPStatus: http.StatusUnprocessableEntity,
	}
	ErrTooManyRequests = &AppError{
		Code:       CodeTooManyRequests,
		Message:    "Too many requests",
		HTTPStatus: http.StatusTooManyRequests,
	}
	ErrInternal = &AppError{
		Code:       CodeInternal,
		Message:    "Internal server error",
		HTTPStatus: http.StatusInternalServerError,
	}
	ErrServiceUnavailable = &AppError{
		Code:       CodeServiceUnavailable,
		Message:    "Service unavailable",
		HTTPStatus: http.StatusServiceUnavailable,
	}
)
