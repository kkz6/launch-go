package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// Create creates a new database
func (r *DatabaseRepository) Create(ctx context.Context, database *models.Database) error {
	return r.installable.Create(ctx, database)
}

// FindByID finds a database by ID
func (r *DatabaseRepository) FindByID(ctx context.Context, id string) (*models.Database, error) {
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
func (r *DatabaseRepository) FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Database, error) {
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

// FindByIDAndTeam finds a database by ID and team ID
func (r *DatabaseRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Database, error) {
	var database models.Database

	err := r.db.WithContext(ctx).
		Preload("Users").
		First(&database, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseNotFound
		}

		return nil, err
	}

	return &database, nil
}

// FindByIDAndServerAndTeam finds a database by ID, server ID, and team ID
func (r *DatabaseRepository) FindByIDAndServerAndTeam(ctx context.Context, id, serverID, teamID string) (*models.Database, error) {
	var database models.Database

	err := r.db.WithContext(ctx).
		Preload("Users").
		First(&database, "id = ? AND server_id = ? AND team_id = ?", id, serverID, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseNotFound
		}

		return nil, err
	}

	return &database, nil
}

// FindByServer finds all databases for a server
func (r *DatabaseRepository) FindByServer(ctx context.Context, serverID string) ([]models.Database, error) {
	var databases []models.Database

	err := r.db.WithContext(ctx).
		Preload("Users").
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&databases).Error

	return databases, err
}

// FindByServerAndTeam finds all databases for a server and team
func (r *DatabaseRepository) FindByServerAndTeam(ctx context.Context, serverID, teamID string) ([]models.Database, error) {
	var databases []models.Database

	err := r.db.WithContext(ctx).
		Preload("Users").
		Where("server_id = ? AND team_id = ?", serverID, teamID).
		Order("created_at DESC").
		Find(&databases).Error

	return databases, err
}

// CountByTeam counts all databases for a team
func (r *DatabaseRepository) CountByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Database{}).
		Where("team_id = ?", teamID).
		Count(&count).Error

	return count, err
}

// FindByNameAndServer finds a database by name and server ID
func (r *DatabaseRepository) FindByNameAndServer(ctx context.Context, name, serverID string) (*models.Database, error) {
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
func (r *DatabaseRepository) FindByUser(ctx context.Context, userID string) ([]models.Database, error) {
	var databases []models.Database

	err := r.db.WithContext(ctx).
		Joins("JOIN database_database_user ON database_database_user.database_id = databases.id").
		Where("database_database_user.database_user_id = ?", userID).
		Find(&databases).Error

	return databases, err
}

// Update updates a database
func (r *DatabaseRepository) Update(ctx context.Context, database *models.Database) error {
	return r.installable.Update(ctx, database)
}

// UpdateFields updates specific fields of a database
func (r *DatabaseRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.installable.UpdateFields(ctx, id, fields)
}

// Delete deletes a database
func (r *DatabaseRepository) Delete(ctx context.Context, id string) error {
	return r.installable.Delete(ctx, id)
}

// ExistsByNameAndServer checks if a database exists with the given name on the server
func (r *DatabaseRepository) ExistsByNameAndServer(ctx context.Context, name, serverID string) (bool, error) {
	return r.installable.ExistsByNameAndServer(ctx, name, serverID)
}

// MarkAsInstalled marks a database as installed
func (r *DatabaseRepository) MarkAsInstalled(ctx context.Context, id string) error {
	return r.installable.MarkAsInstalled(ctx, id)
}

// MarkAsFailed marks a database installation as failed
func (r *DatabaseRepository) MarkAsFailed(ctx context.Context, id string) error {
	return r.installable.MarkAsFailed(ctx, id)
}

// MarkAsUninstalling marks a database as being uninstalled
func (r *DatabaseRepository) MarkAsUninstalling(ctx context.Context, id string) error {
	return r.installable.MarkAsUninstalling(ctx, id)
}

// AttachUser attaches a database user to a database
func (r *DatabaseRepository) AttachUser(ctx context.Context, databaseID, userID string) error {
	return r.db.WithContext(ctx).
		Create(&models.DatabaseDatabaseUser{
			DatabaseID:     databaseID,
			DatabaseUserID: userID,
		}).Error
}

// DetachUser detaches a database user from a database
func (r *DatabaseRepository) DetachUser(ctx context.Context, databaseID, userID string) error {
	return r.db.WithContext(ctx).
		Where("database_id = ? AND database_user_id = ?", databaseID, userID).
		Delete(&models.DatabaseDatabaseUser{}).Error
}

// DetachAllUsers detaches all users from a database
func (r *DatabaseRepository) DetachAllUsers(ctx context.Context, databaseID string) error {
	return r.db.WithContext(ctx).
		Where("database_id = ?", databaseID).
		Delete(&models.DatabaseDatabaseUser{}).Error
}
