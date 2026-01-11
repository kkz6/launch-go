package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// DatabaseRepository defines the interface for database operations
type DatabaseRepository interface {
	Create(ctx context.Context, database *models.Database) error
	FindByID(ctx context.Context, id string) (*models.Database, error)
	FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Database, error)
	FindByServer(ctx context.Context, serverID string) ([]models.Database, error)
	FindByNameAndServer(ctx context.Context, name, serverID string) (*models.Database, error)
	FindByUser(ctx context.Context, userID string) ([]models.Database, error)
	Update(ctx context.Context, database *models.Database) error
	UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	ExistsByNameAndServer(ctx context.Context, name, serverID string) (bool, error)
	AttachUser(ctx context.Context, databaseID, userID string) error
	DetachUser(ctx context.Context, databaseID, userID string) error
	DetachAllUsers(ctx context.Context, databaseID string) error
}

// DatabaseUserRepository defines the interface for database user operations
type DatabaseUserRepository interface {
	CreateUser(ctx context.Context, user *models.DatabaseUser) error
	FindUserByID(ctx context.Context, id string) (*models.DatabaseUser, error)
	FindUserByIDAndServer(ctx context.Context, id, serverID string) (*models.DatabaseUser, error)
	FindUsersByServer(ctx context.Context, serverID string) ([]models.DatabaseUser, error)
	FindUserByNameAndServer(ctx context.Context, name, serverID string) (*models.DatabaseUser, error)
	FindUsersByDatabase(ctx context.Context, databaseID string) ([]models.DatabaseUser, error)
	FindRootUser(ctx context.Context, serverID string) (*models.DatabaseUser, error)
	UpdateUser(ctx context.Context, user *models.DatabaseUser) error
	UpdateUserFields(ctx context.Context, id string, fields map[string]interface{}) error
	DeleteUser(ctx context.Context, id string) error
	UserExistsByNameAndServer(ctx context.Context, name, serverID string) (bool, error)
	SyncUserDatabases(ctx context.Context, userID string, databaseIDs []string) error
}

// Repository combines all repository interfaces
type Repository interface {
	DatabaseRepository
	DatabaseUserRepository
	Transaction(ctx context.Context, fn func(tx Repository) error) error
}
