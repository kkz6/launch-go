package fiber

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/dto"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

// ValidationError is a custom error type that holds field validation errors.
// The global error handler will detect this and format it appropriately.
type ValidationError struct {
	Errors    map[string][]string
	Templates map[string][]ValidationMessage
}

// ValidationMessage defers interpolation until the response locale is known.
type ValidationMessage struct {
	Source string
	Args   []any
}

func (e *ValidationError) Error() string {
	data, _ := json.Marshal(e.errorsForLocale(i18n.DefaultLocale))
	return string(data)
}

// NewValidationError creates a validation error with field-specific messages.
func NewValidationError(errors map[string][]string) *ValidationError {
	return &ValidationError{Errors: errors}
}

// NewValidationErrorMessage creates a parameterized validation error whose
// translated template is formatted only when the HTTP response is rendered.
func NewValidationErrorMessage(field, source string, args ...any) *ValidationError {
	return &ValidationError{Templates: map[string][]ValidationMessage{
		field: {{Source: source, Args: args}},
	}}
}

func (e *ValidationError) errorsForLocale(locale string) map[string][]string {
	if e == nil {
		return nil
	}
	errors := make(map[string][]string, len(e.Errors)+len(e.Templates))
	for field, messages := range e.Errors {
		for _, message := range messages {
			errors[field] = append(errors[field], i18n.Translate(locale, message))
		}
	}
	for field, messages := range e.Templates {
		for _, message := range messages {
			errors[field] = append(errors[field], i18n.Translate(locale, message.Source, message.Args...))
		}
	}
	return errors
}

// LocalizedErrors renders all validation messages for the active request.
func (e *ValidationError) LocalizedErrors(c *fiber.Ctx) map[string][]string {
	return e.errorsForLocale(i18n.Locale(c))
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

	if errs := ValidateStructForLocale(req, i18n.Locale(c)); errs != nil {
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
	if errs := ValidateStructForLocale(&req, i18n.Locale(c)); errs != nil {
		return nil, NewValidationError(errs)
	}
	return &req, nil
}
