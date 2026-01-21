package fiber

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/dto"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// ParseAndValidate parses request body into the provided struct and validates it.
// If the request implements dto.Normalizable, Normalize() is called before validation.
// Returns nil on success, or sends an error response and returns the error.
func ParseAndValidate[T any](c *fiber.Ctx, req *T) error {
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, response.MsgInvalidRequestBody)
	}

	// Call Normalize() if the request implements Normalizable
	if normalizable, ok := any(req).(dto.Normalizable); ok {
		normalizable.Normalize()
	}

	if errs := Validate(req); errs != nil {
		return response.ValidationError(c, errs)
	}
	return nil
}

// MustParseAndValidate parses and validates the request body, returning the parsed struct.
// Returns nil and sends an error response if parsing or validation fails.
func MustParseAndValidate[T any](c *fiber.Ctx) (*T, error) {
	var req T
	if err := ParseAndValidate(c, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// ParseQuery parses query parameters into the provided struct.
// Returns nil on success, or sends an error response and returns the error.
func ParseQuery[T any](c *fiber.Ctx) (*T, error) {
	var req T
	if err := c.QueryParser(&req); err != nil {
		return nil, response.BadRequest(c, response.MsgInvalidQueryParams)
	}
	return &req, nil
}

// ParseQueryWithValidation parses query parameters and validates them.
// Returns nil on success, or sends an error response and returns the error.
func ParseQueryWithValidation[T any](c *fiber.Ctx) (*T, error) {
	var req T
	if err := c.QueryParser(&req); err != nil {
		return nil, response.BadRequest(c, response.MsgInvalidQueryParams)
	}
	if errs := Validate(&req); errs != nil {
		return nil, response.ValidationError(c, errs)
	}
	return &req, nil
}
