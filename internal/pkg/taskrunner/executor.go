package taskrunner

import (
	"context"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

// ServerConnection provides SSH connection details for a server.
// This interface allows the task runner to work without importing server models.
type ServerConnection interface {
	// GetID returns the server's unique identifier
	GetID() string
	// GetTeamID returns the team ID for broadcasting
	GetTeamID() string
	// GetIPAddress returns the server's public IP address
	GetIPAddress() string
	// GetSSHPort returns the SSH port (default 22)
	GetSSHPort() int
	// GetPrivateKey returns the SSH private key
	GetPrivateKey() string
	// GetSudoPassword returns the password for sudo operations
	GetSudoPassword() string
	// GetUsername returns the default username for the server
	GetUsername() string
	// ConnectionAsRoot returns an SSH connection as root
	ConnectionAsRoot() *Connection
	// ConnectionAsUser returns an SSH connection as the given user
	ConnectionAsUser(username ...string) *Connection
}

// ExecutorResult represents the result of a task execution.
type ExecutorResult interface {
	IsSuccessful() bool
	GetOutput() string
	GetExitCode() int
}

// TaskExecutor provides a fluent interface for executing tasks on servers.
// It can be configured with various options before execution.
type TaskExecutor interface {
	// AsRoot configures the task to run with sudo
	AsRoot() TaskExecutor
	// AsUser configures the task to run as the server user
	AsUser() TaskExecutor
	// WithUsername sets a specific username for task execution
	WithUsername(username string) TaskExecutor
	// TrackInDB enables database tracking of task execution
	TrackInDB() TaskExecutor
	// ThrowOnError configures whether to return errors for non-zero exit codes
	ThrowOnError() TaskExecutor
	// Run executes the task synchronously
	Run(ctx context.Context) (ExecutorResult, error)
	// Dispatch executes the task (may be async depending on mode)
	Dispatch(ctx context.Context) (ExecutorResult, error)
}

// ExecutorFactory creates TaskExecutors for running tasks on servers.
type ExecutorFactory interface {
	// NewExecutor creates a new TaskExecutor for the given server and task
	NewExecutor(server ServerConnection, task Task) TaskExecutor
	// RunTask is a convenience method to run a task directly
	RunTask(ctx context.Context, server ServerConnection, task Task, asRoot bool) (ExecutorResult, error)
	// IsLocalMode returns true if running in local development mode
	IsLocalMode() bool
}

// ExecutorDeps holds common dependencies for task execution.
// This can be embedded by module-specific job contexts.
type ExecutorDeps struct {
	DB          *gorm.DB
	Queue       *queue.Client
	Dispatcher  TaskDispatcher
	Logger      *zerolog.Logger
	Broadcaster broadcast.TeamBroadcaster
	Notifier    NotifierService
	LocalMode   bool
}

// IsLocalMode returns true if running in local development mode
func (d *ExecutorDeps) IsLocalMode() bool {
	return d.LocalMode
}
