package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// FindDatabasesByServer returns all databases for a server
func (r *Repository) FindDatabasesByServer(ctx context.Context, serverID string) ([]models.Database, error) {
	return findByServer[models.Database](r, ctx, serverID)
}

// FindDatabaseByID returns a database by ID
func (r *Repository) FindDatabaseByID(ctx context.Context, id string) (*models.Database, error) {
	return findByID[models.Database](r, ctx, id, ErrDatabaseNotFound)
}

// FindDatabaseByIDWithServer returns a database by ID with the Server relation preloaded
func (r *Repository) FindDatabaseByIDWithServer(ctx context.Context, id string) (*models.Database, error) {
	return findByIDWithPreload[models.Database](r, ctx, id, "Server", ErrDatabaseNotFound)
}

// CreateDatabase creates a new database
func (r *Repository) CreateDatabase(ctx context.Context, db *models.Database) error {
	return create(r, ctx, db)
}

// DeleteDatabase deletes a database
func (r *Repository) DeleteDatabase(ctx context.Context, id string) error {
	return deleteByID[models.Database](r, ctx, id)
}

// FindDatabaseUsersByServer returns all database users for a server
func (r *Repository) FindDatabaseUsersByServer(ctx context.Context, serverID string) ([]models.DatabaseUser, error) {
	return findByServer[models.DatabaseUser](r, ctx, serverID)
}

// FindDatabaseUserByID returns a database user by ID
func (r *Repository) FindDatabaseUserByID(ctx context.Context, id string) (*models.DatabaseUser, error) {
	return findByID[models.DatabaseUser](r, ctx, id, ErrDatabaseUserNotFound)
}

// FindDatabaseUserByIDWithServer returns a database user by ID with the Server relation preloaded
func (r *Repository) FindDatabaseUserByIDWithServer(ctx context.Context, id string) (*models.DatabaseUser, error) {
	var user models.DatabaseUser
	err := r.db.WithContext(ctx).Preload("Server").Preload("Databases").Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// CreateDatabaseUser creates a new database user
func (r *Repository) CreateDatabaseUser(ctx context.Context, user *models.DatabaseUser) error {
	return create(r, ctx, user)
}

// DeleteDatabaseUser deletes a database user
func (r *Repository) DeleteDatabaseUser(ctx context.Context, id string) error {
	return deleteByID[models.DatabaseUser](r, ctx, id)
}
