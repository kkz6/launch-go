package fiber

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

var validate *validator.Validate

func init() {
	validate = validator.New()

	// Use JSON tag names in error messages
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

// ValidateStruct validates a struct and returns field-specific errors.
func ValidateStruct(s interface{}) map[string][]string {
	return ValidateStructForLocale(s, i18n.DefaultLocale)
}

// ValidateStructForLocale validates a struct and renders field errors in the
// requested locale.
func ValidateStructForLocale(s interface{}, locale string) map[string][]string {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	errors := make(map[string][]string)

	for _, err := range err.(validator.ValidationErrors) {
		field := err.Field()
		errors[field] = append(errors[field], getValidationErrorMessage(locale, err))
	}

	return errors
}

func getValidationErrorMessage(locale string, err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return i18n.Translate(locale, "This field is required")
	case "email":
		return i18n.Translate(locale, "Must be a valid email address")
	case "min":
		return i18n.Translate(locale, "Must be at least %s characters", err.Param())
	case "max":
		return i18n.Translate(locale, "Must be at most %s characters", err.Param())
	case "oneof":
		return i18n.Translate(locale, "Must be one of: %s", err.Param())
	case "url":
		return i18n.Translate(locale, "Must be a valid URL")
	case "uuid":
		return i18n.Translate(locale, "Must be a valid UUID")
	case "len":
		return i18n.Translate(locale, "Must be exactly %s characters", err.Param())
	case "ulid":
		return i18n.Translate(locale, "Must be a valid ULID")
	case "eqfield":
		return i18n.Translate(locale, "Must match the %s field", err.Param())
	case "fqdn":
		return i18n.Translate(locale, "Must be a valid domain name")
	case "ip":
		return i18n.Translate(locale, "Must be a valid IP address")
	case "cidr":
		return i18n.Translate(locale, "Must be a valid CIDR notation")
	case "required_if", "required_unless", "required_without", "required_with":
		return i18n.Translate(locale, "This field is required")
	case "eq":
		return i18n.Translate(locale, "Must equal %s", err.Param())
	default:
		return i18n.Translate(locale, "Invalid value")
	}
}
