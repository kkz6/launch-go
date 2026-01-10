package errors

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound          = errors.New("resource not found")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrValidation        = errors.New("validation error")
	ErrInternal          = errors.New("internal server error")
	ErrBadRequest        = errors.New("bad request")
	ErrConflict          = errors.New("resource conflict")
	ErrSSHConnection     = errors.New("ssh connection failed")
	ErrProviderAPI       = errors.New("cloud provider api error")
	ErrDeploymentFailed  = errors.New("deployment failed")
	ErrServerUnavailable = errors.New("server unavailable")
)

type AppError struct {
	Err     error
	Message string
	Code    int
}

func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Err.Error()
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(err error, message string, code int) *AppError {
	return &AppError{
		Err:     err,
		Message: message,
		Code:    code,
	}
}

func Wrap(err error, message string) error {
	return fmt.Errorf("%s: %w", message, err)
}

func Is(err, target error) bool {
	return errors.Is(err, target)
}
