package jobs

import (
	"context"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// ===============================
// Model Interfaces for Trackers
// ===============================

// Installable is implemented by models that can track installation status.
// This provides compile-time safety for installation tracking.
//
// Models should implement:
//   - GetID() string - returns the model's primary key
//   - TableName() string - returns the database table name
type Installable interface {
	GetID() string
	TableName() string
}

// Uninstallable is implemented by models that can be uninstalled/deleted.
// Use this with UninstallationTracker to ensure type safety.
type Uninstallable interface {
	GetID() string
	TableName() string
}

// StatusTrackable is implemented by models that have a status field.
// Use this with StatusTracker for type-safe status updates.
type StatusTrackable interface {
	GetID() string
	TableName() string
}

// TaskTrackable is implemented by models that track server task IDs.
// Use this with TaskTracker for type-safe task ID management.
type TaskTrackable interface {
	GetID() string
	TableName() string
}

// ===============================
// Job Capability Interfaces
// ===============================

// DependencyAware is implemented by jobs that need the standard dependencies.
// Use this interface to ensure a job has access to required services.
type DependencyAware interface {
	GetDB() *gorm.DB
	GetLogger() *zerolog.Logger
	GetWS() Broadcaster
	GetDispatcher() *taskrunner.Dispatcher
	GetQueue() *queue.Client
}

// DependencySetter is implemented by jobs that can receive dependencies.
type DependencySetter interface {
	SetDB(db *gorm.DB)
	SetLogger(logger *zerolog.Logger)
	SetWS(ws Broadcaster)
	SetDispatcher(dispatcher *taskrunner.Dispatcher)
	SetQueue(q *queue.Client)
}

// ===============================
// Server Connection Interface
// ===============================

// ServerConnectable represents a server that can be connected to via SSH.
// This abstraction allows jobs to work with server models without
// importing the server module directly.
type ServerConnectable interface {
	// GetID returns the server's ID
	GetID() string

	// GetPublicIPv4 returns the server's public IPv4 address
	GetPublicIPv4() string

	// GetPrivateKey returns the server's private SSH key
	GetPrivateKey() string

	// GetUsername returns the default SSH username
	GetUsername() string

	// ConnectionAsRoot returns an SSH connection as root
	ConnectionAsRoot() *taskrunner.Connection

	// ConnectionAsUser returns an SSH connection as the default user
	// or optionally as a specified username
	ConnectionAsUser(username ...string) *taskrunner.Connection
}

// ===============================
// Repository Interfaces
// ===============================

// GenericRepository is a generic interface for basic CRUD operations.
type GenericRepository[T any] interface {
	FindByID(ctx context.Context, id string) (T, error)
	Create(ctx context.Context, model T) error
	Update(ctx context.Context, model T) error
	Delete(ctx context.Context, id string) error
}

// ===============================
// Job Handler Interfaces
// ===============================

// HandlerWithPayload is a Handler that also exposes its payload type.
// This is useful for jobs that need to access their payload generically.
type HandlerWithPayload[P any] interface {
	Handler
	GetPayload() P
}

// FailableJob is a job that can handle failures.
// All jobs implementing Handler should also implement this implicitly
// through the Failed method.
type FailableJob interface {
	Failed(ctx context.Context, err error)
}

// ===============================
// Event Broadcasting
// ===============================

// ServerEventBroadcaster can broadcast events to server channels.
type ServerEventBroadcaster interface {
	BroadcastToServer(serverID, event string, data interface{})
}

// SiteEventBroadcaster can broadcast events to site channels.
type SiteEventBroadcaster interface {
	BroadcastToSite(siteID, event string, data interface{})
}

// DeploymentEventBroadcaster can broadcast events to deployment channels.
type DeploymentEventBroadcaster interface {
	BroadcastToDeployment(deploymentID, event string, data interface{})
}

// FullBroadcaster combines all broadcasting capabilities.
type FullBroadcaster interface {
	Broadcaster
	ServerEventBroadcaster
	SiteEventBroadcaster
	DeploymentEventBroadcaster
}

// ===============================
// Compile-time Interface Checks
// ===============================

// These ensure BaseJob implements the required interfaces.
var (
	_ DependencyAware = (*BaseJob)(nil)
)
