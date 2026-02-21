package enumtypes

import (
	"database/sql/driver"
	"fmt"
)

// ScanString is a helper for implementing sql.Scanner for string enums.
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

// ScanStringWithDefault is like ScanString but uses a custom default when value is nil.
func ScanStringWithDefault[T ~string](dest *T, value interface{}, defaultVal T) error {
	if value == nil {
		*dest = defaultVal
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

// ValueString is a helper for implementing driver.Valuer for string enums.
func ValueString[T ~string](v T) (driver.Value, error) {
	return string(v), nil
}

// ScanInt is a helper for implementing sql.Scanner for int enums.
func ScanInt[T ~int](dest *T, value interface{}) error {
	if value == nil {
		*dest = 0
		return nil
	}

	switch v := value.(type) {
	case int64:
		*dest = T(v)
	case int:
		*dest = T(v)
	default:
		return fmt.Errorf("cannot scan type %T into int enum", value)
	}

	return nil
}

// ValueInt is a helper for implementing driver.Valuer for int enums.
func ValueInt[T ~int](v T) (driver.Value, error) {
	return int64(v), nil
}

// Scan provides a generic Scan implementation for string-based enums.
// This function handles nil, string, and []byte types from database drivers.
// Alias for ScanString for backwards compatibility.
func Scan[E ~string](e *E, value any) error {
	return ScanString(e, value)
}

// Value provides a generic Value implementation for string-based enums.
// Alias for ValueString for backwards compatibility.
func Value[E ~string](e E) (driver.Value, error) {
	return ValueString(e)
}

// ParseEnum is a generic function to parse a string into an enum value.
// It validates against the provided list of valid values.
func ParseEnum[T ~string](s string, valid []T) (T, error) {
	v := T(s)
	for _, val := range valid {
		if v == val {
			return v, nil
		}
	}

	var zero T
	return zero, fmt.Errorf("invalid enum value: %s", s)
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
