package repository

import (
	"errors"
	"fmt"
	"net/http"
)

// Common repository errors
var (
	ErrNotFound     = errors.New("record not found")
	ErrDuplicate    = errors.New("duplicate record")
	ErrInvalidData  = errors.New("invalid data")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrServerError  = errors.New("internal server error")
)

// ModelError is an error that includes context about which model/resource failed
type ModelError struct {
	Err     error
	Model   string
	ID      string
	Message string
	Status  int
}

// Error implements the error interface
func (e *ModelError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.ID != "" {
		return fmt.Sprintf("%s with ID '%s' not found", e.Model, e.ID)
	}
	return e.Err.Error()
}

// Unwrap allows errors.Is and errors.As to work
func (e *ModelError) Unwrap() error {
	return e.Err
}

// HTTPStatus returns the HTTP status code for this error
func (e *ModelError) HTTPStatus() int {
	if e.Status != 0 {
		return e.Status
	}
	switch {
	case errors.Is(e.Err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(e.Err, ErrDuplicate):
		return http.StatusConflict
	case errors.Is(e.Err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(e.Err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(e.Err, ErrInvalidData):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// NotFoundError creates a not found error for a model
func NotFoundError(model string, id string) *ModelError {
	return &ModelError{
		Err:     ErrNotFound,
		Model:   model,
		ID:      id,
		Message: fmt.Sprintf("%s not found", model),
		Status:  http.StatusNotFound,
	}
}

// NotFoundErrorf creates a not found error with custom message
func NotFoundErrorf(format string, args ...interface{}) *ModelError {
	return &ModelError{
		Err:     ErrNotFound,
		Message: fmt.Sprintf(format, args...),
		Status:  http.StatusNotFound,
	}
}

// WrapError wraps an error with model context
func WrapError(err error, model string, message string) *ModelError {
	return &ModelError{
		Err:     err,
		Model:   model,
		Message: message,
	}
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsDuplicate checks if an error is a duplicate error
func IsDuplicate(err error) bool {
	return errors.Is(err, ErrDuplicate)
}
