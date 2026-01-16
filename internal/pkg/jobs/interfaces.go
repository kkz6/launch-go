package jobs

import (
	"context"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

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
