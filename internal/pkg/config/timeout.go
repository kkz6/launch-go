// Package timeout provides centralized timeout configuration values
// used throughout the application. This ensures consistency and makes
// timeout values easy to configure and maintain.
package config

import "time"

// Standard timeout values used throughout the application.
// These are package-level variables to allow for configuration overrides.
var (
	// SSH is the default timeout for SSH connections
	SSH = 30 * time.Second

	// HTTP is the default timeout for HTTP requests
	HTTP = 30 * time.Second

	// Fiber is the default read timeout for Fiber HTTP server
	Fiber = 30 * time.Second

	// TaskDefault is the default timeout for task execution
	TaskDefault = 10 * time.Minute

	// NetworkDial is the timeout for network dial operations
	NetworkDial = 5 * time.Second

	// RetryDelay is the delay between retry attempts
	RetryDelay = 10 * time.Second

	// LongTask is the timeout for long-running tasks like provisioning
	LongTask = 30 * time.Minute

	// DatabaseQuery is the default timeout for database queries
	DatabaseQuery = 30 * time.Second

	// CacheOperation is the default timeout for cache operations
	CacheOperation = 5 * time.Second

	// WebhookDelivery is the timeout for webhook HTTP requests
	WebhookDelivery = 30 * time.Second

	// SSLCertificateCheck is the timeout for SSL certificate operations
	SSLCertificateCheck = 60 * time.Second
)

// Config holds configurable timeout values that can be loaded from
// environment variables or configuration files.
type Config struct {
	SSH           time.Duration
	HTTP          time.Duration
	Fiber         time.Duration
	TaskDefault   time.Duration
	NetworkDial   time.Duration
	RetryDelay    time.Duration
	LongTask      time.Duration
	DatabaseQuery time.Duration
}

// Default returns the default timeout configuration.
func Default() *Config {
	return &Config{
		SSH:           SSH,
		HTTP:          HTTP,
		Fiber:         Fiber,
		TaskDefault:   TaskDefault,
		NetworkDial:   NetworkDial,
		RetryDelay:    RetryDelay,
		LongTask:      LongTask,
		DatabaseQuery: DatabaseQuery,
	}
}

// Apply applies the configuration values to the package-level defaults.
// This is useful for overriding defaults from environment variables.
func (c *Config) Apply() {
	if c.SSH > 0 {
		SSH = c.SSH
	}

	if c.HTTP > 0 {
		HTTP = c.HTTP
	}

	if c.Fiber > 0 {
		Fiber = c.Fiber
	}

	if c.TaskDefault > 0 {
		TaskDefault = c.TaskDefault
	}

	if c.NetworkDial > 0 {
		NetworkDial = c.NetworkDial
	}

	if c.RetryDelay > 0 {
		RetryDelay = c.RetryDelay
	}

	if c.LongTask > 0 {
		LongTask = c.LongTask
	}

	if c.DatabaseQuery > 0 {
		DatabaseQuery = c.DatabaseQuery
	}
}

// ForTask returns an appropriate timeout based on the task type.
// This provides a convenient way to get task-specific timeouts.
func ForTask(taskType string) time.Duration {
	switch taskType {
	case "server:provision", "server:install_database":
		return LongTask
	case "site:deploy", "site:deploy_zero_downtime":
		return TaskDefault
	default:
		return TaskDefault
	}
}
