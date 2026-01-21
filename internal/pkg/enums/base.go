package enums

import (
	"database/sql/driver"
	"fmt"
)

// StringEnum is the interface that all string-based enums should implement.
// This interface defines the common methods expected of enum types.
type StringEnum interface {
	~string
	String() string
	IsValid() bool
}

// LabeledEnum extends StringEnum with a human-readable label.
type LabeledEnum interface {
	StringEnum
	Label() string
}

// EnumDefinition holds metadata about an enum type including its values and labels.
// This is useful for building UIs, validation rules, and API documentation.
type EnumDefinition[T ~string] struct {
	values []T
	labels map[T]string
}

// NewEnumDefinition creates a new enum definition with the given values and labels.
// Labels are optional - if not provided, the string value will be used as the label.
func NewEnumDefinition[T ~string](values []T, labels map[T]string) EnumDefinition[T] {
	return EnumDefinition[T]{
		values: values,
		labels: labels,
	}
}

// Values returns all valid values for this enum.
func (d EnumDefinition[T]) Values() []T {
	return d.values
}

// Label returns the human-readable label for the given value.
// If no label is defined, returns the string value itself.
func (d EnumDefinition[T]) Label(v T) string {
	if label, ok := d.labels[v]; ok {
		return label
	}
	return string(v)
}

// IsValid checks if the given value is a valid enum value.
func (d EnumDefinition[T]) IsValid(v T) bool {
	for _, valid := range d.values {
		if v == valid {
			return true
		}
	}
	return false
}

// Parse attempts to parse a string into an enum value.
// Returns an error if the value is not valid.
func (d EnumDefinition[T]) Parse(s string) (T, error) {
	v := T(s)
	if !d.IsValid(v) {
		var zero T
		return zero, fmt.Errorf("invalid enum value: %s", s)
	}
	return v, nil
}

// ParseWithDefault attempts to parse a string into an enum value.
// Returns the default value if the string is not a valid enum value.
func (d EnumDefinition[T]) ParseWithDefault(s string, defaultVal T) T {
	v := T(s)
	if !d.IsValid(v) {
		return defaultVal
	}
	return v
}

// Contains checks if the slice contains the given value.
func Contains[T ~string](values []T, v T) bool {
	for _, val := range values {
		if val == v {
			return true
		}
	}
	return false
}

// ParseEnum is a generic function to parse a string into an enum value.
// It validates against the provided list of valid values.
func ParseEnum[T ~string](s string, valid []T) (T, error) {
	v := T(s)
	if !Contains(valid, v) {
		var zero T
		return zero, fmt.Errorf("invalid enum value: %s", s)
	}
	return v, nil
}

// ParseEnumWithDefault is a generic function to parse a string into an enum value.
// Returns the default value if parsing fails.
func ParseEnumWithDefault[T ~string](s string, valid []T, defaultVal T) T {
	v := T(s)
	if !Contains(valid, v) {
		return defaultVal
	}
	return v
}

// ScanString is a helper function for implementing sql.Scanner for string enums.
// It handles nil, string, and []byte types.
func ScanString[T ~string](dest *T, value interface{}) error {
	if value == nil {
		*dest = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*dest = T(v)
	case []byte:
		*dest = T(string(v))
	default:
		return fmt.Errorf("cannot scan type %T into enum", value)
	}

	return nil
}

// ValueString is a helper function for implementing driver.Valuer for string enums.
func ValueString[T ~string](v T) (driver.Value, error) {
	return string(v), nil
}

// StringValues returns the string representations of enum values.
func StringValues[T ~string](values []T) []string {
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = string(v)
	}
	return result
}

// MapLabels creates a map of enum values to their labels.
// Useful for serialization and UI rendering.
func MapLabels[T LabeledEnum](values []T) map[string]string {
	result := make(map[string]string, len(values))
	for _, v := range values {
		result[v.String()] = v.Label()
	}
	return result
}

// Filter returns a new slice containing only values that match the predicate.
func Filter[T ~string](values []T, predicate func(T) bool) []T {
	var result []T
	for _, v := range values {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

// First returns the first value that matches the predicate, or the zero value if none match.
func First[T ~string](values []T, predicate func(T) bool) T {
	for _, v := range values {
		if predicate(v) {
			return v
		}
	}
	var zero T
	return zero
}
