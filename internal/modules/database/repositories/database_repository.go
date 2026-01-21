package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
)

// FindByID finds a database by ID
func (r *DatabaseRepository) FindByID(ctx context.Context, id string) (*models.Database, error) {
	var database models.Database

	err := r.DB.WithContext(ctx).
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

	err := r.DB.WithContext(ctx).
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

	err := r.DB.WithContext(ctx).
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

	err := r.DB.WithContext(ctx).
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

	err := r.DB.WithContext(ctx).
		Preload("Users").
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&databases).Error

	return databases, err
}

// FindByServerAndTeam finds all databases for a server and team
func (r *DatabaseRepository) FindByServerAndTeam(ctx context.Context, serverID, teamID string) ([]models.Database, error) {
	var databases []models.Database

	err := r.DB.WithContext(ctx).
		Preload("Users").
		Where("server_id = ? AND team_id = ?", serverID, teamID).
		Order("created_at DESC").
		Find(&databases).Error

	return databases, err
}

// CountByTeam counts all databases for a team
func (r *DatabaseRepository) CountByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Database{}).
		Where("team_id = ?", teamID).
		Count(&count).Error

	return count, err
}

// FindByNameAndServer finds a database by name and server ID
func (r *DatabaseRepository) FindByNameAndServer(ctx context.Context, name, serverID string) (*models.Database, error) {
	var database models.Database

	err := r.DB.WithContext(ctx).
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

	err := r.DB.WithContext(ctx).
		Joins("JOIN database_database_user ON database_database_user.database_id = databases.id").
		Where("database_database_user.database_user_id = ?", userID).
		Find(&databases).Error

	return databases, err
}

// AttachUser attaches a database user to a database
func (r *DatabaseRepository) AttachUser(ctx context.Context, databaseID, userID string) error {
	return r.DB.WithContext(ctx).
		Create(&models.DatabaseDatabaseUser{
			DatabaseID:     databaseID,
			DatabaseUserID: userID,
		}).Error
}

// DetachUser detaches a database user from a database
func (r *DatabaseRepository) DetachUser(ctx context.Context, databaseID, userID string) error {
	return r.DB.WithContext(ctx).
		Where("database_id = ? AND database_user_id = ?", databaseID, userID).
		Delete(&models.DatabaseDatabaseUser{}).Error
}

// DetachAllUsers detaches all users from a database
func (r *DatabaseRepository) DetachAllUsers(ctx context.Context, databaseID string) error {
	return r.DB.WithContext(ctx).
		Where("database_id = ?", databaseID).
		Delete(&models.DatabaseDatabaseUser{}).Error
}
