package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// DatabaseService defines the interface for database business logic
type DatabaseService interface {
	CreateDatabase(ctx context.Context, serverID string, req *dto.CreateDatabaseRequest, userID *string) (*models.Database, error)
	GetDatabase(ctx context.Context, id, serverID string) (*models.Database, error)
	ListDatabases(ctx context.Context, serverID string) ([]models.Database, error)
	DeleteDatabase(ctx context.Context, id, serverID string, userID *string) error
	SyncDatabases(ctx context.Context, serverID string, userID *string) error
	BroadcastDatabaseStatus(serverID, databaseID, status, message string)
}

// DatabaseUserService defines the interface for database user business logic
type DatabaseUserService interface {
	CreateDatabaseUser(ctx context.Context, serverID string, req *dto.CreateDatabaseUserRequest, userID *string) (*models.DatabaseUser, error)
	GetDatabaseUser(ctx context.Context, id, serverID string) (*models.DatabaseUser, error)
	ListDatabaseUsers(ctx context.Context, serverID string) ([]models.DatabaseUser, error)
	UpdateDatabaseUser(ctx context.Context, id, serverID string, req *dto.UpdateDatabaseUserRequest, userID *string) (*models.DatabaseUser, error)
	DeleteDatabaseUser(ctx context.Context, id, serverID string, userID *string) error
	BroadcastDatabaseUserStatus(serverID, userID, status, message string)
}

// Service combines all service interfaces
type Service interface {
	DatabaseService
	DatabaseUserService
}
