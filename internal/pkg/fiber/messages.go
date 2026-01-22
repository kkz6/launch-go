package fiber

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
const (
	MsgInternalError      = "An unexpected error occurred"
	MsgInvalidCredentials = "Invalid credentials"
	MsgInvalidToken       = "Invalid token"
	MsgValidationFailed   = "Validation failed"
)
