package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// Create creates a new database
func (r *Repository) Create(ctx context.Context, database *models.Database) error {
	return r.db.WithContext(ctx).Create(database).Error
}

// FindByID finds a database by ID
func (r *Repository) FindByID(ctx context.Context, id string) (*models.Database, error) {
	var database models.Database

	err := r.db.WithContext(ctx).
		Preload("Users").
		First(&database, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseNotFound
		}

		return nil, err
	}

	return &database, nil
}

// FindByIDAndServer finds a database by ID and server ID
func (r *Repository) FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Database, error) {
	var database models.Database

	err := r.db.WithContext(ctx).
		Preload("Users").
		First(&database, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseNotFound
		}

		return nil, err
	}

	return &database, nil
}

// FindByServer finds all databases for a server
func (r *Repository) FindByServer(ctx context.Context, serverID string) ([]models.Database, error) {
	var databases []models.Database

	err := r.db.WithContext(ctx).
		Preload("Users").
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&databases).Error

	return databases, err
}

// FindByNameAndServer finds a database by name and server ID
func (r *Repository) FindByNameAndServer(ctx context.Context, name, serverID string) (*models.Database, error) {
	var database models.Database

	err := r.db.WithContext(ctx).
		Preload("Users").
		First(&database, "name = ? AND server_id = ?", name, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseNotFound
		}

		return nil, err
	}

	return &database, nil
}

// FindByUser finds all databases for a database user
func (r *Repository) FindByUser(ctx context.Context, userID string) ([]models.Database, error) {
	var databases []models.Database

	err := r.db.WithContext(ctx).
		Joins("JOIN database_database_user ON database_database_user.database_id = databases.id").
		Where("database_database_user.database_user_id = ?", userID).
		Find(&databases).Error

	return databases, err
}

// Update updates a database
func (r *Repository) Update(ctx context.Context, database *models.Database) error {
	return r.db.WithContext(ctx).Save(database).Error
}

// UpdateFields updates specific fields of a database
func (r *Repository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.Database{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete deletes a database
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Database{}, "id = ?", id).Error
}

// ExistsByNameAndServer checks if a database exists with the given name on the server
func (r *Repository) ExistsByNameAndServer(ctx context.Context, name, serverID string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&models.Database{}).
		Where("name = ? AND server_id = ?", name, serverID).
		Count(&count).Error

	return count > 0, err
}

// AttachUser attaches a database user to a database
func (r *Repository) AttachUser(ctx context.Context, databaseID, userID string) error {
	return r.db.WithContext(ctx).
		Create(&models.DatabaseDatabaseUser{
			DatabaseID:     databaseID,
			DatabaseUserID: userID,
		}).Error
}

// DetachUser detaches a database user from a database
func (r *Repository) DetachUser(ctx context.Context, databaseID, userID string) error {
	return r.db.WithContext(ctx).
		Where("database_id = ? AND database_user_id = ?", databaseID, userID).
		Delete(&models.DatabaseDatabaseUser{}).Error
}

// DetachAllUsers detaches all users from a database
func (r *Repository) DetachAllUsers(ctx context.Context, databaseID string) error {
	return r.db.WithContext(ctx).
		Where("database_id = ?", databaseID).
		Delete(&models.DatabaseDatabaseUser{}).Error
}
