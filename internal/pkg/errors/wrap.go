package errors

import (
	"errors"
	"fmt"
)

// Wrapf wraps an error with a formatted context message.
// If err is nil, returns nil.
//
// Example:
//
//	if err := db.Create(&user).Error; err != nil {
//	    return errors.Wrapf(err, "failed to create user %s", userID)
//	}
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// WrapWithCode wraps an error with context and an HTTP status code
func WrapWithCode(err error, message string, code int) *ResourceError {
	return &ResourceError{
		Message: fmt.Sprintf("%s: %v", message, err),
		Status:  code,
	}
}

// Unwrap returns the underlying error if it exists
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// As finds the first error in err's chain that matches target
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Join returns an error that wraps the given errors.
// Any nil error values are discarded.
func Join(errs ...error) error {
	return errors.Join(errs...)
}

// Newf creates a new error with formatted text
func Newf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// Must panics if err is not nil, otherwise returns value.
// Use for initialization code where errors are unrecoverable.
//
// Example:
//
//	cfg := errors.Must(config.Load())
func Must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}

// MustNoErr panics if err is not nil.
// Use for initialization code where errors are unrecoverable.
//
// Example:
//
//	errors.MustNoErr(db.AutoMigrate(&User{}))
func MustNoErr(err error) {
	if err != nil {
		panic(err)
	}
}

// Ignore discards the error and returns only the value.
// Use sparingly and only when you're certain the error can be ignored.
//
// Example:
//
//	count := errors.Ignore(strconv.Atoi(s))
func Ignore[T any](val T, _ error) T {
	return val
}

// IgnoreErr discards an error return value.
// Use sparingly and document why the error is being ignored.
//
// Example:
//
//	errors.IgnoreErr(file.Close()) // best effort cleanup
func IgnoreErr(_ error) {}

// WithStack wraps an error with stack trace information.
// Note: For full stack traces, consider using pkg/errors or similar.
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w", err)
}

// Cause returns the root cause of an error by unwrapping all layers
func Cause(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}

// IsAny checks if err matches any of the target errors
func IsAny(err error, targets ...error) bool {
	for _, target := range targets {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
