package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// CreateUser creates a new database user
func (r *Repository) CreateUser(ctx context.Context, user *models.DatabaseUser) error {
	return r.user.Create(ctx, user)
}

// FindUserByID finds a database user by ID
func (r *Repository) FindUserByID(ctx context.Context, id string) (*models.DatabaseUser, error) {
	var user models.DatabaseUser

	err := r.db.WithContext(ctx).
		Preload("Databases").
		First(&user, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseUserNotFound
		}

		return nil, err
	}

	return &user, nil
}

// FindUserByIDAndServer finds a database user by ID and server ID
func (r *Repository) FindUserByIDAndServer(ctx context.Context, id, serverID string) (*models.DatabaseUser, error) {
	var user models.DatabaseUser

	err := r.db.WithContext(ctx).
		Preload("Databases").
		First(&user, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseUserNotFound
		}

		return nil, err
	}

	return &user, nil
}

// FindUsersByServer finds all database users for a server
func (r *Repository) FindUsersByServer(ctx context.Context, serverID string) ([]models.DatabaseUser, error) {
	var users []models.DatabaseUser

	err := r.db.WithContext(ctx).
		Preload("Databases").
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&users).Error

	return users, err
}

// FindUserByNameAndServer finds a database user by name and server ID
func (r *Repository) FindUserByNameAndServer(ctx context.Context, name, serverID string) (*models.DatabaseUser, error) {
	var user models.DatabaseUser

	err := r.db.WithContext(ctx).
		Preload("Databases").
		First(&user, "name = ? AND server_id = ?", name, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseUserNotFound
		}

		return nil, err
	}

	return &user, nil
}

// FindUsersByDatabase finds all database users for a database
func (r *Repository) FindUsersByDatabase(ctx context.Context, databaseID string) ([]models.DatabaseUser, error) {
	var users []models.DatabaseUser

	err := r.db.WithContext(ctx).
		Joins("JOIN database_database_user ON database_database_user.database_user_id = database_users.id").
		Where("database_database_user.database_id = ?", databaseID).
		Find(&users).Error

	return users, err
}

// UpdateUser updates a database user
func (r *Repository) UpdateUser(ctx context.Context, user *models.DatabaseUser) error {
	return r.user.Update(ctx, user)
}

// UpdateUserFields updates specific fields of a database user
func (r *Repository) UpdateUserFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.user.UpdateFields(ctx, id, fields)
}

// DeleteUser deletes a database user
func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	return r.user.Delete(ctx, id)
}

// UserExistsByNameAndServer checks if a database user exists with the given name on the server
func (r *Repository) UserExistsByNameAndServer(ctx context.Context, name, serverID string) (bool, error) {
	return r.user.ExistsByNameAndServer(ctx, name, serverID)
}

// MarkUserAsInstalled marks a database user as installed
func (r *Repository) MarkUserAsInstalled(ctx context.Context, id string) error {
	return r.user.MarkAsInstalled(ctx, id)
}

// MarkUserAsFailed marks a database user installation as failed
func (r *Repository) MarkUserAsFailed(ctx context.Context, id string) error {
	return r.user.MarkAsFailed(ctx, id)
}

// MarkUserAsUninstalling marks a database user as being uninstalled
func (r *Repository) MarkUserAsUninstalling(ctx context.Context, id string) error {
	return r.user.MarkAsUninstalling(ctx, id)
}

// SyncUserDatabases syncs the databases attached to a user
func (r *Repository) SyncUserDatabases(ctx context.Context, userID string, databaseIDs []string) error {
	// Delete existing associations
	if err := r.db.WithContext(ctx).
		Where("database_user_id = ?", userID).
		Delete(&models.DatabaseDatabaseUser{}).Error; err != nil {
		return err
	}

	// Create new associations
	for _, dbID := range databaseIDs {
		if err := r.db.WithContext(ctx).
			Create(&models.DatabaseDatabaseUser{
				DatabaseID:     dbID,
				DatabaseUserID: userID,
			}).Error; err != nil {
			return err
		}
	}

	return nil
}

// FindRootUser finds the root user for a server
func (r *Repository) FindRootUser(ctx context.Context, serverID string) (*models.DatabaseUser, error) {
	return r.FindUserByNameAndServer(ctx, "root", serverID)
}
