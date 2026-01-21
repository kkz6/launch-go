package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// =============================================================================
// Error Wrapping Functions
// =============================================================================

// Wrap wraps an error with a context message.
// If err is nil, returns nil.
//
// Example:
//
//	if err := db.Create(&user).Error; err != nil {
//	    return errors.Wrap(err, "failed to create user")
//	}
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

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

// WrapWithCode wraps an error with context and an HTTP status code.
// Returns a ResourceError that implements HTTPStatusError.
//
// Example:
//
//	return errors.WrapWithCode(err, "failed to fetch user", http.StatusNotFound)
func WrapWithCode(err error, message string, code int) *ResourceError {
	if err == nil {
		return nil
	}
	return &ResourceError{
		Message: fmt.Sprintf("%s: %v", message, err),
		Status:  code,
	}
}

// WrapNotFound wraps an error as a not found error.
// Convenience function for WrapWithCode with 404 status.
func WrapNotFound(err error, message string) *ResourceError {
	return WrapWithCode(err, message, http.StatusNotFound)
}

// WrapBadRequest wraps an error as a bad request error.
// Convenience function for WrapWithCode with 400 status.
func WrapBadRequest(err error, message string) *ResourceError {
	return WrapWithCode(err, message, http.StatusBadRequest)
}

// WrapConflict wraps an error as a conflict error.
// Convenience function for WrapWithCode with 409 status.
func WrapConflict(err error, message string) *ResourceError {
	return WrapWithCode(err, message, http.StatusConflict)
}

// WrapInternal wraps an error as an internal server error.
// Convenience function for WrapWithCode with 500 status.
func WrapInternal(err error, message string) *ResourceError {
	return WrapWithCode(err, message, http.StatusInternalServerError)
}

// WrapAsAppError wraps an error with full AppError context.
// This provides the most structured error response.
//
// Example:
//
//	return errors.WrapAsAppError(err, "user_creation_failed", "Failed to create user", 500)
func WrapAsAppError(err error, code string, message string, statusCode int) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{
		Cause:      err,
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// =============================================================================
// Error Unwrapping Functions
// =============================================================================

// Unwrap returns the underlying error if it exists.
// Alias for errors.Unwrap for convenience.
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// Cause returns the root cause of an error by unwrapping all layers.
//
// Example:
//
//	rootErr := errors.Cause(wrappedErr)
func Cause(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}

// Join returns an error that wraps the given errors.
// Any nil error values are discarded.
// Alias for errors.Join for convenience.
func Join(errs ...error) error {
	return errors.Join(errs...)
}

// =============================================================================
// Error Creation Functions
// =============================================================================

// Newf creates a new error with formatted text.
//
// Example:
//
//	return errors.Newf("user %s not found", userID)
func Newf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// WithStack wraps an error preserving the error chain.
// Note: For full stack traces, consider using pkg/errors or similar.
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w", err)
}

// =============================================================================
// Utility Functions
// =============================================================================

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

// OrElse returns the value if err is nil, otherwise returns the fallback.
// Useful for providing default values on error.
//
// Example:
//
//	count := errors.OrElse(strconv.Atoi(s), 0)
func OrElse[T any](val T, err error, fallback T) T {
	if err != nil {
		return fallback
	}
	return val
}

// OrDefault returns the value if err is nil, otherwise returns the zero value.
//
// Example:
//
//	count := errors.OrDefault(strconv.Atoi(s))
func OrDefault[T any](val T, err error) T {
	if err != nil {
		var zero T
		return zero
	}
	return val
}
