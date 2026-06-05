package fiber

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/dto"
)

// ValidationError is a custom error type that holds field validation errors.
// The global error handler will detect this and format it appropriately.
type ValidationError struct {
	Errors map[string][]string
}

func (e *ValidationError) Error() string {
	data, _ := json.Marshal(e.Errors)
	return string(data)
}

// NewValidationError creates a validation error with field-specific messages.
func NewValidationError(errors map[string][]string) *ValidationError {
	return &ValidationError{Errors: errors}
}

// ParseAndValidate parses request body into the provided struct and validates it.
// If the request implements dto.Normalizable, Normalize() is called before validation.
// Returns nil on success, or a fiber error that will be handled by the error handler.
func ParseAndValidate[T any](c *fiber.Ctx, req *T) error {
	if err := c.BodyParser(req); err != nil {
		return BadRequest("Invalid request body")
	}

	// Call Normalize() if the request implements Normalizable
	if normalizable, ok := any(req).(dto.Normalizable); ok {
		normalizable.Normalize()
	}

	if errs := ValidateStruct(req); errs != nil {
		return NewValidationError(errs)
	}
	return nil
}

// MustParseAndValidate parses and validates the request body, returning the parsed struct.
// Returns nil and an error if parsing or validation fails.
func MustParseAndValidate[T any](c *fiber.Ctx) (*T, error) {
	var req T
	if err := ParseAndValidate(c, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// ParseQuery parses query parameters into the provided struct.
// Returns nil on success, or an error if parsing fails.
func ParseQuery[T any](c *fiber.Ctx) (*T, error) {
	var req T
	if err := c.QueryParser(&req); err != nil {
		return nil, BadRequest("Invalid query parameters")
	}
	return &req, nil
}

// ParseQueryWithValidation parses query parameters and validates them.
// Returns nil on success, or an error if parsing or validation fails.
func ParseQueryWithValidation[T any](c *fiber.Ctx) (*T, error) {
	var req T
	if err := c.QueryParser(&req); err != nil {
		return nil, BadRequest("Invalid query parameters")
	}
	if errs := ValidateStruct(&req); errs != nil {
		return nil, NewValidationError(errs)
	}
	return &req, nil
}
