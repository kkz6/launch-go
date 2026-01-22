package fiber

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

// Default error messages
const (
	MsgNotFound     = "Resource not found"
	MsgUnauthorized = "Unauthorized"
	MsgForbidden    = "Access denied"
	MsgBadRequest   = "Bad request"
	MsgConflict     = "Resource conflict"
	MsgInternal     = "Internal server error"
	MsgValidation   = "Validation failed"
	MsgTooMany      = "Too many requests"
)

// Sentinel errors for errors.Is() checks in business logic
var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
)

// NotFound returns a 404 error with optional custom message
func NotFound(message ...string) error {
	return fiber.NewError(fiber.StatusNotFound, msgOrDefault(message, MsgNotFound))
}

// Unauthorized returns a 401 error with optional custom message
func Unauthorized(message ...string) error {
	return fiber.NewError(fiber.StatusUnauthorized, msgOrDefault(message, MsgUnauthorized))
}

// Forbidden returns a 403 error with optional custom message
func Forbidden(message ...string) error {
	return fiber.NewError(fiber.StatusForbidden, msgOrDefault(message, MsgForbidden))
}

// BadRequest returns a 400 error with optional custom message
func BadRequest(message ...string) error {
	return fiber.NewError(fiber.StatusBadRequest, msgOrDefault(message, MsgBadRequest))
}

// Conflict returns a 409 error with optional custom message
func Conflict(message ...string) error {
	return fiber.NewError(fiber.StatusConflict, msgOrDefault(message, MsgConflict))
}

// Internal returns a 500 error with optional custom message
func Internal(message ...string) error {
	return fiber.NewError(fiber.StatusInternalServerError, msgOrDefault(message, MsgInternal))
}

// Validation returns a 422 error with optional custom message
func Validation(message ...string) error {
	return fiber.NewError(fiber.StatusUnprocessableEntity, msgOrDefault(message, MsgValidation))
}

// TooManyRequests returns a 429 error with optional custom message
func TooManyRequests(message ...string) error {
	return fiber.NewError(fiber.StatusTooManyRequests, msgOrDefault(message, MsgTooMany))
}

func msgOrDefault(message []string, defaultMsg string) string {
	if len(message) > 0 && message[0] != "" {
		return message[0]
	}
	return defaultMsg
}

// IsNotFound checks if error is a not found condition
func IsNotFound(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var e *fiber.Error
	if errors.As(err, &e) && e.Code == fiber.StatusNotFound {
		return true
	}
	return false
}

// IsUnauthorized checks if error is an unauthorized condition
func IsUnauthorized(err error) bool {
	var e *fiber.Error
	if errors.As(err, &e) && e.Code == fiber.StatusUnauthorized {
		return true
	}
	return false
}

// IsForbidden checks if error is a forbidden condition
func IsForbidden(err error) bool {
	var e *fiber.Error
	if errors.As(err, &e) && e.Code == fiber.StatusForbidden {
		return true
	}
	return false
}

// IsConflict checks if error is a conflict condition
func IsConflict(err error) bool {
	var e *fiber.Error
	if errors.As(err, &e) && e.Code == fiber.StatusConflict {
		return true
	}
	return false
}

// IsValidationError checks if error is a validation error
func IsValidationError(err error) bool {
	if errors.Is(err, ErrValidation) {
		return true
	}
	var e *fiber.Error
	if errors.As(err, &e) {
		return e.Code == fiber.StatusBadRequest || e.Code == fiber.StatusUnprocessableEntity
	}
	return false
}

// IsInternalError checks if error is a server error (5xx)
func IsInternalError(err error) bool {
	var e *fiber.Error
	if errors.As(err, &e) && e.Code >= 500 {
		return true
	}
	return false
}
