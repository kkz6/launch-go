package errors

import (
	"errors"
	"net/http"
)

// =============================================================================
// Sentinel Errors
// =============================================================================
// These are the base error types that can be checked with errors.Is().
// Use these for error type checking, not for returning to clients directly.

var (
	// General errors
	ErrNotFound        = errors.New("resource not found")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrValidation      = errors.New("validation error")
	ErrInternal        = errors.New("internal server error")
	ErrBadRequest      = errors.New("bad request")
	ErrConflict        = errors.New("resource conflict")
	ErrTimeout         = errors.New("operation timed out")
	ErrCanceled        = errors.New("operation canceled")
	ErrUnavailable     = errors.New("service unavailable")
	ErrTooManyRequests = errors.New("too many requests")
	ErrUnprocessable   = errors.New("unprocessable entity")

	// Infrastructure errors
	ErrSSHConnection     = errors.New("ssh connection failed")
	ErrProviderAPI       = errors.New("cloud provider api error")
	ErrServerUnavailable = errors.New("server unavailable")

	// Domain-specific errors
	ErrDeploymentFailed   = errors.New("deployment failed")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrExpiredToken       = errors.New("token expired")
	ErrInvalidToken       = errors.New("invalid token")
)

// =============================================================================
// Pre-defined ResourceErrors for common HTTP error responses
// =============================================================================
// These are ready-to-use error instances that can be returned directly from
// handlers and services. They implement HTTPStatusError.

// Server module errors
var (
	ErrServerNotFound         = NotFound("Server not found")
	ErrServiceNotFound        = NotFound("Service not found")
	ErrFirewallRuleNotFound   = NotFound("Firewall rule not found")
	ErrCronNotFound           = NotFound("Cron job not found")
	ErrDaemonNotFound         = NotFound("Daemon not found")
	ErrSSHKeyNotFound         = NotFound("SSH key not found")
	ErrTaskNotFound           = NotFound("Task not found")
	ErrMetricNotFound         = NotFound("Metric not found")
	ErrServerProviderNotFound = NotFound("Server provider not found")
)

// Site module errors
var (
	ErrSiteNotFound        = NotFound("Site not found")
	ErrDeploymentNotFound  = NotFound("Deployment not found")
	ErrQueueNotFound       = NotFound("Queue not found")
	ErrCertificateNotFound = NotFound("Certificate not found")
	ErrRedirectNotFound    = NotFound("Redirect not found")
	ErrCommandNotFound     = NotFound("Command not found")
	ErrReleaseNotFound     = NotFound("Release not found")
)

// Database module errors
var (
	ErrDatabaseNotFound     = NotFound("Database not found")
	ErrDatabaseUserNotFound = NotFound("Database user not found")
)

// Git module errors
var (
	ErrSourceControlNotFound = NotFound("Source control not found")
	ErrRepositoryNotFound    = NotFound("Repository not found")
)

// Backup module errors
var (
	ErrBackupNotFound          = NotFound("Backup not found")
	ErrStorageProviderNotFound = NotFound("Storage provider not found")
	ErrBackupJobNotFound       = NotFound("Backup job not found")
)

// DNS module errors
var (
	ErrDNSProviderNotFound = NotFound("Provider not found")
	ErrDomainNotFound      = NotFound("Domain not found")
	ErrDNSRecordNotFound   = NotFound("Record not found")
)

// Notification module errors
var (
	ErrNotificationChannelNotFound = NotFound("Notification channel not found")
)

// Billing module errors
var (
	ErrSubscriptionNotFound = NotFound("Subscription not found")
	ErrPlanNotFound         = NotFound("Plan not found")
)

// Auth module errors
var (
	ErrUserNotFound = NotFound("User not found")
	ErrTeamNotFound = NotFound("Team not found")
)

// Script module errors
var (
	ErrScriptNotFound    = NotFound("Script not found")
	ErrExecutionNotFound = NotFound("Script execution not found")
)

// =============================================================================
// Pre-defined AppErrors with error codes
// =============================================================================
// These can be used for more structured error responses with error codes.

var (
	// Authentication errors
	AppErrUnauthorized = &AppError{
		Code:       "unauthorized",
		Message:    "Authentication required",
		StatusCode: http.StatusUnauthorized,
	}

	AppErrForbidden = &AppError{
		Code:       "forbidden",
		Message:    "Access denied",
		StatusCode: http.StatusForbidden,
	}

	AppErrInvalidCredentials = &AppError{
		Code:       "invalid_credentials",
		Message:    "Invalid email or password",
		StatusCode: http.StatusUnauthorized,
	}

	AppErrTokenExpired = &AppError{
		Code:       "token_expired",
		Message:    "Token has expired",
		StatusCode: http.StatusUnauthorized,
	}

	// Validation errors
	AppErrValidation = &AppError{
		Code:       "validation_error",
		Message:    "Validation failed",
		StatusCode: http.StatusUnprocessableEntity,
	}

	AppErrBadRequest = &AppError{
		Code:       "bad_request",
		Message:    "Invalid request",
		StatusCode: http.StatusBadRequest,
	}

	// Resource errors
	AppErrNotFound = &AppError{
		Code:       "not_found",
		Message:    "Resource not found",
		StatusCode: http.StatusNotFound,
	}

	AppErrConflict = &AppError{
		Code:       "conflict",
		Message:    "Resource conflict",
		StatusCode: http.StatusConflict,
	}

	// Server errors
	AppErrInternal = &AppError{
		Code:       "internal_error",
		Message:    "An internal error occurred",
		StatusCode: http.StatusInternalServerError,
	}

	AppErrUnavailable = &AppError{
		Code:       "service_unavailable",
		Message:    "Service temporarily unavailable",
		StatusCode: http.StatusServiceUnavailable,
	}

	// Rate limiting
	AppErrTooManyRequests = &AppError{
		Code:       "too_many_requests",
		Message:    "Too many requests, please try again later",
		StatusCode: http.StatusTooManyRequests,
	}
)

// =============================================================================
// Error Checking Helpers
// =============================================================================

// Is reports whether any error in err's chain matches target.
// Alias for errors.Is for convenience.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target.
// Alias for errors.As for convenience.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// IsNotFound checks if an error represents a "not found" condition.
// It checks for ErrNotFound sentinel error and ResourceError with 404 status.
func IsNotFound(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}

	var resErr *ResourceError
	if errors.As(err, &resErr) && resErr.Status == http.StatusNotFound {
		return true
	}

	var appErr *AppError
	if errors.As(err, &appErr) && appErr.StatusCode == http.StatusNotFound {
		return true
	}

	return false
}

// IsUnauthorized checks if an error represents an "unauthorized" condition.
func IsUnauthorized(err error) bool {
	if errors.Is(err, ErrUnauthorized) {
		return true
	}

	var httpErr HTTPStatusError
	if errors.As(err, &httpErr) && httpErr.HTTPStatus() == http.StatusUnauthorized {
		return true
	}

	return false
}

// IsForbidden checks if an error represents a "forbidden" condition.
func IsForbidden(err error) bool {
	if errors.Is(err, ErrForbidden) {
		return true
	}

	var httpErr HTTPStatusError
	if errors.As(err, &httpErr) && httpErr.HTTPStatus() == http.StatusForbidden {
		return true
	}

	return false
}

// IsConflict checks if an error represents a "conflict" condition.
func IsConflict(err error) bool {
	if errors.Is(err, ErrConflict) {
		return true
	}

	var httpErr HTTPStatusError
	if errors.As(err, &httpErr) && httpErr.HTTPStatus() == http.StatusConflict {
		return true
	}

	return false
}

// IsValidationError checks if an error represents a validation error.
func IsValidationError(err error) bool {
	if errors.Is(err, ErrValidation) {
		return true
	}

	var httpErr HTTPStatusError
	if errors.As(err, &httpErr) {
		status := httpErr.HTTPStatus()
		return status == http.StatusBadRequest || status == http.StatusUnprocessableEntity
	}

	return false
}

// IsInternalError checks if an error represents an internal server error.
func IsInternalError(err error) bool {
	if errors.Is(err, ErrInternal) {
		return true
	}

	var httpErr HTTPStatusError
	if errors.As(err, &httpErr) && httpErr.HTTPStatus() >= 500 {
		return true
	}

	return false
}

// IsAny checks if err matches any of the target errors.
func IsAny(err error, targets ...error) bool {
	for _, target := range targets {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
