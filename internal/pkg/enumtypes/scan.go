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
