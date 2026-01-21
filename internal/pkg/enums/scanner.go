package enums

import (
	"database/sql/driver"
	"errors"
)

// ErrInvalidEnumScan is returned when an unsupported type is passed to Scan.
var ErrInvalidEnumScan = errors.New("invalid type for enum scan")

// Scan provides a generic Scan implementation for string-based enums.
// This function handles nil, string, and []byte types from database drivers.
//
// Usage in enum types:
//
//	func (s *ServerStatus) Scan(value any) error {
//	    return enums.Scan(s, value)
//	}
func Scan[E ~string](e *E, value any) error {
	if value == nil {
		*e = E("")
		return nil
	}

	switch v := value.(type) {
	case string:
		*e = E(v)
	case []byte:
		*e = E(string(v))
	default:
		return ErrInvalidEnumScan
	}

	return nil
}

// Value provides a generic Value implementation for string-based enums.
// This function converts the enum to its string representation for database storage.
//
// Usage in enum types:
//
//	func (s ServerStatus) Value() (driver.Value, error) {
//	    return enums.Value(s)
//	}
func Value[E ~string](e E) (driver.Value, error) {
	return string(e), nil
}

// String provides a generic String implementation for string-based enums.
//
// Usage in enum types:
//
//	func (s ServerStatus) String() string {
//	    return enums.String(s)
//	}
func String[E ~string](e E) string {
	return string(e)
}

// ScanNullable provides a Scan implementation that handles NULL values gracefully.
// Unlike Scan, this returns true for the second value when the scanned value was NULL.
//
// Usage in enum types that need to track NULL vs empty string:
//
//	func (s *ServerStatus) ScanNullable(value any) (bool, error) {
//	    return enums.ScanNullable(s, value)
//	}
func ScanNullable[E ~string](e *E, value any) (isNull bool, err error) {
	if value == nil {
		*e = E("")
		return true, nil
	}

	switch v := value.(type) {
	case string:
		*e = E(v)
	case []byte:
		*e = E(string(v))
	default:
		return false, ErrInvalidEnumScan
	}

	return false, nil
}

// ValueNullable provides a Value implementation that returns NULL for empty strings.
// This is useful when you want to store empty enums as NULL in the database.
//
// Usage in enum types:
//
//	func (s ServerStatus) ValueNullable() (driver.Value, error) {
//	    return enums.ValueNullable(s)
//	}
func ValueNullable[E ~string](e E) (driver.Value, error) {
	if string(e) == "" {
		return nil, nil
	}

	return string(e), nil
}

// IsEmpty returns true if the enum value is empty.
func IsEmpty[E ~string](e E) bool {
	return string(e) == ""
}

// Equals compares two enum values for equality.
func Equals[E ~string](a, b E) bool {
	return string(a) == string(b)
}

// In checks if the enum value is in the provided list of values.
func In[E ~string](e E, values ...E) bool {
	for _, v := range values {
		if e == v {
			return true
		}
	}

	return false
}

// NotIn checks if the enum value is not in the provided list of values.
func NotIn[E ~string](e E, values ...E) bool {
	return !In(e, values...)
}
