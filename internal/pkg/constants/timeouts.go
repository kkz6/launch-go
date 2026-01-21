package constants

import "time"

// Timeout constants for various operations.
const (
	// DefaultHTTPTimeout is the default timeout for HTTP requests.
	DefaultHTTPTimeout = 30 * time.Second

	// DefaultHTTPLongTimeout is for longer HTTP operations like file uploads.
	DefaultHTTPLongTimeout = 2 * time.Minute

	// DefaultSSHTimeout is the default timeout for SSH connections.
	DefaultSSHTimeout = 30 * time.Second

	// DefaultSSHCommandTimeout is the default timeout for SSH commands.
	DefaultSSHCommandTimeout = 2 * time.Minute

	// DefaultSSHLongCommandTimeout is for long-running SSH commands.
	DefaultSSHLongCommandTimeout = 10 * time.Minute

	// DefaultDeployTimeout is the default timeout for deployments.
	DefaultDeployTimeout = 10 * time.Minute

	// DefaultProvisionTimeout is the default timeout for server provisioning.
	DefaultProvisionTimeout = 30 * time.Minute

	// DefaultDatabaseTimeout is the default timeout for database operations.
	DefaultDatabaseTimeout = 5 * time.Second

	// DefaultCacheTimeout is the default TTL for cached items.
	DefaultCacheTimeout = 5 * time.Minute

	// DefaultSessionTimeout is the default session expiration.
	DefaultSessionTimeout = 24 * time.Hour

	// DefaultTokenExpiry is the default access token expiration.
	DefaultTokenExpiry = 1 * time.Hour

	// DefaultRefreshTokenExpiry is the default refresh token expiration.
	DefaultRefreshTokenExpiry = 7 * 24 * time.Hour

	// DefaultWebhookTimeout is the timeout for webhook deliveries.
	DefaultWebhookTimeout = 10 * time.Second

	// DefaultHealthCheckInterval is the interval between health checks.
	DefaultHealthCheckInterval = 30 * time.Second

	// DefaultRetryDelay is the default delay between retries.
	DefaultRetryDelay = 1 * time.Second

	// DefaultMaxRetryDelay is the maximum delay between retries.
	DefaultMaxRetryDelay = 30 * time.Second
)

// Retry constants for exponential backoff.
const (
	// DefaultMaxRetries is the default maximum number of retry attempts.
	DefaultMaxRetries = 3

	// DefaultRetryMultiplier is the multiplier for exponential backoff.
	DefaultRetryMultiplier = 2.0
)
