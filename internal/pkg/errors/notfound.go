package errors

import (
	"net/http"
)

// ResourceError represents an HTTP-aware error for a specific resource
type ResourceError struct {
	Message string
	Status  int
}

// Error implements the error interface
func (e *ResourceError) Error() string {
	return e.Message
}

// HTTPStatus returns the HTTP status code for this error
func (e *ResourceError) HTTPStatus() int {
	return e.Status
}

// NotFound creates a new not found error with the given message
func NotFound(message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  http.StatusNotFound,
	}
}

// BadRequest creates a new bad request error with the given message
func BadRequest(message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  http.StatusBadRequest,
	}
}

// Conflict creates a new conflict error with the given message
func Conflict(message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  http.StatusConflict,
	}
}

// Internal creates a new internal server error with the given message
func Internal(message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  http.StatusInternalServerError,
	}
}

// Forbidden creates a new forbidden error with the given message
func Forbidden(message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  http.StatusForbidden,
	}
}

// Unauthorized creates a new unauthorized error with the given message
func Unauthorized(message string) *ResourceError {
	return &ResourceError{
		Message: message,
		Status:  http.StatusUnauthorized,
	}
}

// Common "not found" errors used across modules
// Server module errors
var (
	ErrServerNotFound         = NotFound("Server not found")
	ErrServiceNotFound        = NotFound("Service not found")
	ErrFirewallRuleNotFound   = NotFound("Firewall rule not found")
	ErrCronNotFound           = NotFound("Cron job not found")
	ErrDaemonNotFound         = NotFound("Daemon not found")
	ErrSshKeyNotFound         = NotFound("SSH key not found")
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
