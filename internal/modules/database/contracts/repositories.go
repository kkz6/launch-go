package contracts

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// DatabaseRepository defines the interface for database operations
type DatabaseRepository interface {
	Create(ctx context.Context, database *models.Database) error
	FindByID(ctx context.Context, id string) (*models.Database, error)
	FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Database, error)
	FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Database, error)
	FindByIDAndServerAndTeam(ctx context.Context, id, serverID, teamID string) (*models.Database, error)
	FindByServer(ctx context.Context, serverID string) ([]models.Database, error)
	FindByServerAndTeam(ctx context.Context, serverID, teamID string) ([]models.Database, error)
	FindByNameAndServer(ctx context.Context, name, serverID string) (*models.Database, error)
	FindByUser(ctx context.Context, userID string) ([]models.Database, error)
	CountByTeam(ctx context.Context, teamID string) (int64, error)
	Update(ctx context.Context, database *models.Database) error
	UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	ExistsByNameAndServer(ctx context.Context, name, serverID string) (bool, error)
	MarkAsInstalled(ctx context.Context, id string) error
	MarkAsFailed(ctx context.Context, id string) error
	MarkAsUninstalling(ctx context.Context, id string) error
	AttachUser(ctx context.Context, databaseID, userID string) error
	DetachUser(ctx context.Context, databaseID, userID string) error
	DetachAllUsers(ctx context.Context, databaseID string) error
}

// DatabaseUserRepository defines the interface for database user operations
type DatabaseUserRepository interface {
	Create(ctx context.Context, user *models.DatabaseUser) error
	FindByID(ctx context.Context, id string) (*models.DatabaseUser, error)
	FindByIDAndServer(ctx context.Context, id, serverID string) (*models.DatabaseUser, error)
	FindByServer(ctx context.Context, serverID string) ([]models.DatabaseUser, error)
	FindByNameAndServer(ctx context.Context, name, serverID string) (*models.DatabaseUser, error)
	FindByDatabase(ctx context.Context, databaseID string) ([]models.DatabaseUser, error)
	FindRootUser(ctx context.Context, serverID string) (*models.DatabaseUser, error)
	Update(ctx context.Context, user *models.DatabaseUser) error
	UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	ExistsByNameAndServer(ctx context.Context, name, serverID string) (bool, error)
	MarkAsInstalled(ctx context.Context, id string) error
	MarkAsFailed(ctx context.Context, id string) error
	MarkAsUninstalling(ctx context.Context, id string) error
	SyncDatabases(ctx context.Context, userID string, databaseIDs []string) error
}

// RepositoryRegistry provides access to all repositories
type RepositoryRegistry interface {
	Database() DatabaseRepository
	User() DatabaseUserRepository
	DB() *gorm.DB
}
