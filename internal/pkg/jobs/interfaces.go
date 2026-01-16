package jobs

import (
	"context"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

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

// GenericRepository is a generic interface for basic CRUD operations.
type GenericRepository[T any] interface {
	FindByID(ctx context.Context, id string) (T, error)
	Create(ctx context.Context, model T) error
	Update(ctx context.Context, model T) error
	Delete(ctx context.Context, id string) error
}


// ServerEventBroadcaster can broadcast events to server channels.
type ServerEventBroadcaster interface {
	BroadcastToServer(serverID, event string, data any)
}

// SiteEventBroadcaster can broadcast events to site channels.
type SiteEventBroadcaster interface {
	BroadcastToSite(siteID, event string, data any)
}

// DeploymentEventBroadcaster can broadcast events to deployment channels.
type DeploymentEventBroadcaster interface {
	BroadcastToDeployment(deploymentID, event string, data any)
}

// FullBroadcaster combines all broadcasting capabilities.
type FullBroadcaster interface {
	Broadcaster
	ServerEventBroadcaster
	SiteEventBroadcaster
	DeploymentEventBroadcaster
}
