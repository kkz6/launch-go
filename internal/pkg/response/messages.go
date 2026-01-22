package response

// Standard success messages for consistent API responses.
const (
	// CRUD operations
	MsgCreated   = "Resource created successfully"
	MsgUpdated   = "Resource updated successfully"
	MsgDeleted   = "Resource deleted successfully"
	MsgRetrieved = "Resource retrieved successfully"

	// Actions
	MsgActionCompleted = "Action completed successfully"
	MsgStarted         = "Operation started"
	MsgStopped         = "Operation stopped"
	MsgRestarted       = "Operation restarted"
)

// Standard request parsing messages.
const (
	MsgInvalidRequestBody    = "Invalid request body"
	MsgInvalidQueryParams    = "Invalid query parameters"
	MsgMissingRequiredParams = "Missing required parameters"
)

// Standard error messages for handlers.
// These are kept for backwards compatibility.
const (
	MsgInternalError      = "An unexpected error occurred"
	MsgUnauthorized       = "Unauthorized"
	MsgForbidden          = "Access denied"
	MsgInvalidCredentials = "Invalid credentials"
	MsgInvalidToken       = "Invalid token"
	MsgValidationFailed   = "Validation failed"
)

// Generic not found messages - use these instead of module-specific ones.
const (
	MsgNotFound                = "Resource not found"
	MsgResourceNotFound        = "Resource not found"
	MsgServerNotFound          = "Server not found"
	MsgSiteNotFound            = "Site not found"
	MsgSiteNotInstalled        = "Site is not installed"
	MsgUserNotFound            = "User not found"
	MsgTeamNotFound            = "Team not found"
	MsgDeploymentNotFound      = "Deployment not found"
	MsgDatabaseNotFound        = "Database not found"
	MsgBackupNotFound          = "Backup not found"
	MsgDomainNotFound          = "Domain not found"
	MsgDNSRecordNotFound       = "DNS record not found"
	MsgSourceControlNotFound   = "Source control not found"
	MsgStorageProviderNotFound = "Storage provider not found"
	MsgQueueNotFound           = "Queue not found"
	MsgRedirectNotFound        = "Redirect not found"
)
