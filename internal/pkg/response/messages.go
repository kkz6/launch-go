package response

// Standard error messages for consistent API responses.
// Use these constants instead of hardcoding strings in handlers.
const (
	// Request parsing errors
	MsgInvalidRequestBody    = "Invalid request body"
	MsgInvalidQueryParams    = "Invalid query parameters"
	MsgMissingRequiredParams = "Missing required parameters"

	// Validation errors
	MsgValidationFailed = "Validation failed"

	// Authentication/Authorization errors
	MsgUnauthorized     = "Unauthorized"
	MsgForbidden        = "Access denied"
	MsgInvalidToken     = "Invalid token"
	MsgTokenExpired     = "Token expired"
	MsgInvalidCredentials = "Invalid credentials"

	// Resource errors
	MsgResourceNotFound    = "Resource not found"
	MsgResourceConflict    = "Resource already exists"
	MsgResourceUnavailable = "Resource unavailable"

	// Server errors
	MsgInternalError = "An unexpected error occurred"
	MsgServiceError  = "Service temporarily unavailable"

	// Rate limiting
	MsgRateLimitExceeded = "Rate limit exceeded"

	// Request timeout
	MsgRequestTimeout    = "Request timeout"
	MsgRequestCancelled  = "Request cancelled"
)

// Standard success messages for consistent API responses.
const (
	// CRUD operations
	MsgCreated  = "Resource created successfully"
	MsgUpdated  = "Resource updated successfully"
	MsgDeleted  = "Resource deleted successfully"
	MsgRetrieved = "Resource retrieved successfully"

	// Actions
	MsgActionCompleted = "Action completed successfully"
	MsgStarted         = "Operation started"
	MsgStopped         = "Operation stopped"
	MsgRestarted       = "Operation restarted"
)

// Module-specific error messages.
// These are commonly used across handlers in specific modules.
const (
	// Server module
	MsgServerNotFound         = "Server not found"
	MsgServerNotProvisioned   = "Server is not provisioned"
	MsgServerNotConnected     = "Server is not connected"
	MsgServiceNotFound        = "Service not found"
	MsgFirewallRuleNotFound   = "Firewall rule not found"
	MsgCronNotFound           = "Cron job not found"
	MsgDaemonNotFound         = "Daemon not found"
	MsgSSHKeyNotFound         = "SSH key not found"

	// Site module
	MsgSiteNotFound         = "Site not found"
	MsgSiteNotInstalled     = "Site is not installed"
	MsgDeploymentNotFound   = "Deployment not found"
	MsgQueueNotFound        = "Queue not found"
	MsgCertificateNotFound  = "Certificate not found"
	MsgRedirectNotFound     = "Redirect not found"
	MsgDeploymentInProgress = "Deployment already in progress"

	// Database module
	MsgDatabaseNotFound     = "Database not found"
	MsgDatabaseUserNotFound = "Database user not found"

	// Backup module
	MsgBackupNotFound          = "Backup not found"
	MsgStorageProviderNotFound = "Storage provider not found"

	// Git module
	MsgSourceControlNotFound = "Source control not found"
	MsgRepositoryNotFound    = "Repository not found"

	// DNS module
	MsgDomainNotFound    = "Domain not found"
	MsgDNSRecordNotFound = "DNS record not found"

	// Auth module
	MsgUserNotFound = "User not found"
	MsgTeamNotFound = "Team not found"
	MsgInvitationNotFound = "Invitation not found"
	MsgAlreadyTeamMember = "User is already a team member"
)
