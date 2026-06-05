package repositories

import (
	"context"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DatabaseRepository handles database-related database operations for the server module
type DatabaseRepository struct {
	repository.Base[models.Database]
}

// NewDatabaseRepository creates a new DatabaseRepository instance
func NewDatabaseRepository(db *gorm.DB) *DatabaseRepository {
	return &DatabaseRepository{
		Base: repository.NewBase[models.Database](db),
	}
}

// FindByID returns a database by ID
func (r *DatabaseRepository) FindByID(ctx context.Context, id string) (*models.Database, error) {
	database, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return database, nil
}

// FindByIDWithServer returns a database by ID with the Server relation preloaded
func (r *DatabaseRepository) FindByIDWithServer(ctx context.Context, id string) (*models.Database, error) {
	return repository.FindOne[models.Database](ctx, r.DB,
		repository.WithID(id),
		repository.Preload("Server"),
	)
}

// FindUsersByServer returns all database users for a server
func (r *DatabaseRepository) FindUsersByServer(ctx context.Context, serverID string) ([]models.DatabaseUser, error) {
	return repository.FindAll[models.DatabaseUser](ctx, r.DB,
		repository.WithServerID(serverID),
		repository.OrderByCreatedDesc(),
	)
}

// FindUserByID returns a database user by ID
func (r *DatabaseRepository) FindUserByID(ctx context.Context, id string) (*models.DatabaseUser, error) {
	return repository.FindOne[models.DatabaseUser](ctx, r.DB, repository.WithID(id))
}

// FindUserByIDWithServer returns a database user by ID with the Server relation preloaded
func (r *DatabaseRepository) FindUserByIDWithServer(ctx context.Context, id string) (*models.DatabaseUser, error) {
	return repository.FindOne[models.DatabaseUser](ctx, r.DB,
		repository.WithID(id),
		repository.Preload("Server"),
		repository.Preload("Databases"),
	)
}

// CreateUser creates a new database user
func (r *DatabaseRepository) CreateUser(ctx context.Context, user *models.DatabaseUser) error {
	return r.DB.WithContext(ctx).Create(user).Error
}

// DeleteUser deletes a database user
func (r *DatabaseRepository) DeleteUser(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.DatabaseUser{}, "id = ?", id).Error
}
