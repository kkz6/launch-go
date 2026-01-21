package constants

// Pagination limits for API endpoints.
const (
	// DefaultPaginationLimit is the default number of items per page.
	DefaultPaginationLimit = 15

	// MaxPaginationLimit is the maximum number of items per page.
	MaxPaginationLimit = 100

	// DefaultAPIPageSize is used for internal API calls (e.g., provider APIs).
	DefaultAPIPageSize = 100

	// MaxAPIPageSize is the maximum page size for provider APIs.
	MaxAPIPageSize = 200
)

// Upload and file size limits.
const (
	// MaxUploadSize is the maximum file upload size (100 MB).
	MaxUploadSize = 100 * 1024 * 1024

	// MaxRequestBodySize is the maximum request body size (10 MB).
	MaxRequestBodySize = 10 * 1024 * 1024

	// MaxLogFileSize is the maximum log file size before rotation (50 MB).
	MaxLogFileSize = 50 * 1024 * 1024

	// MaxBackupFileSize is the maximum backup file size (5 GB).
	MaxBackupFileSize = 5 * 1024 * 1024 * 1024
)

// Query and result limits.
const (
	// DefaultTasksLimit is the default limit for task listings.
	DefaultTasksLimit = 50

	// DefaultMetricsLimit is the default limit for metrics queries.
	DefaultMetricsLimit = 100

	// MaxDeploymentsToKeep is the maximum number of deployments to retain.
	MaxDeploymentsToKeep = 10

	// MaxReleasesToKeep is the maximum number of releases to retain.
	MaxReleasesToKeep = 5

	// MaxLogsToKeep is the maximum number of log entries to retain.
	MaxLogsToKeep = 1000
)

// Rate limiting constants.
const (
	// DefaultRateLimitPerMinute is the default rate limit per minute.
	DefaultRateLimitPerMinute = 60

	// DefaultRateLimitPerHour is the default rate limit per hour.
	DefaultRateLimitPerHour = 1000

	// AuthRateLimitPerMinute is the rate limit for authentication endpoints.
	AuthRateLimitPerMinute = 10

	// WebhookRateLimitPerMinute is the rate limit for webhook endpoints.
	WebhookRateLimitPerMinute = 100
)

// String length limits.
const (
	// MaxNameLength is the maximum length for resource names.
	MaxNameLength = 100

	// MaxDescriptionLength is the maximum length for descriptions.
	MaxDescriptionLength = 500

	// MaxCommandLength is the maximum length for shell commands.
	MaxCommandLength = 10000

	// MaxScriptLength is the maximum length for scripts.
	MaxScriptLength = 100000

	// MaxURLLength is the maximum length for URLs.
	MaxURLLength = 2048
)

// Retry limits.
const (
	// MaxRetryAttempts is the default maximum number of retry attempts.
	MaxRetryAttempts = 3

	// MaxWebhookRetries is the maximum number of webhook delivery retries.
	MaxWebhookRetries = 5

	// MaxProvisioningRetries is the maximum number of provisioning retries.
	MaxProvisioningRetries = 2
)

// Buffer sizes.
const (
	// DefaultBufferSize is the default buffer size for I/O operations.
	DefaultBufferSize = 32 * 1024

	// SSHBufferSize is the buffer size for SSH output.
	SSHBufferSize = 64 * 1024

	// WebSocketBufferSize is the buffer size for WebSocket messages.
	WebSocketBufferSize = 16 * 1024
)

// ByteUnit constants for clarity.
const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
)
