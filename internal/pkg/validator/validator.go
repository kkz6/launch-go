package validator

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/kkz6/launch-go/internal/pkg/response"
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

func Validate(s interface{}) response.ValidationErrors {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	errors := make(response.ValidationErrors)

	for _, err := range err.(validator.ValidationErrors) {
		field := err.Field()
		errors[field] = append(errors[field], getErrorMessage(err))
	}

	return errors
}

func getErrorMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Must be a valid email address"
	case "min":
		return "Must be at least " + err.Param() + " characters"
	case "max":
		return "Must be at most " + err.Param() + " characters"
	case "oneof":
		return "Must be one of: " + err.Param()
	case "url":
		return "Must be a valid URL"
	case "uuid":
		return "Must be a valid UUID"
	default:
		return "Invalid value"
	}
}
