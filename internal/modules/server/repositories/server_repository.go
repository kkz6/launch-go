package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CreateServer creates a new server
func (r *Repository) CreateServer(ctx context.Context, server *models.Server) error {
	return r.db.WithContext(ctx).Create(server).Error
}

// FindServerByID finds a server by ID
func (r *Repository) FindServerByID(ctx context.Context, id string) (*models.Server, error) {
	var server models.Server
	err := r.db.WithContext(ctx).
		Preload("Services").
		First(&server, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServerNotFound
		}

		return nil, err
	}

	return &server, nil
}

// FindServerByIDAndTeam finds a server by ID and team ID
func (r *Repository) FindServerByIDAndTeam(ctx context.Context, id, teamID string) (*models.Server, error) {
	var server models.Server
	err := r.db.WithContext(ctx).
		Preload("Services").
		First(&server, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServerNotFound
		}

		return nil, err
	}

	return &server, nil
}

// FindServerWithRelations finds a server with all relations
func (r *Repository) FindServerWithRelations(ctx context.Context, id, teamID string) (*models.Server, error) {
	var server models.Server
	err := r.db.WithContext(ctx).
		Preload("Services").
		Preload("FirewallRules").
		Preload("Crons").
		Preload("Daemons").
		Preload("SshKeys").
		First(&server, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServerNotFound
		}

		return nil, err
	}

	return &server, nil
}

// FindAllServersByTeam finds all active (non-archived) servers for a team
func (r *Repository) FindAllServersByTeam(ctx context.Context, teamID string) ([]models.Server, error) {
	var servers []models.Server
	err := r.db.WithContext(ctx).
		Preload("Services").
		Where("team_id = ? AND archived_at IS NULL", teamID).
		Order("created_at DESC").
		Find(&servers).Error

	return servers, err
}

// FindAllServersByTeamPaginated finds all servers for a team with pagination
func (r *Repository) FindAllServersByTeamPaginated(ctx context.Context, teamID string, limit, offset int) ([]models.Server, int64, error) {
	var servers []models.Server
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Server{}).Where("team_id = ?", teamID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Preload("Services").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&servers).Error

	return servers, total, err
}

// FindArchivedServersByTeam finds all archived servers for a team
func (r *Repository) FindArchivedServersByTeam(ctx context.Context, teamID string) ([]models.Server, error) {
	var servers []models.Server
	err := r.db.WithContext(ctx).
		Unscoped().
		Where("team_id = ? AND archived_at IS NOT NULL", teamID).
		Order("archived_at DESC").
		Find(&servers).Error

	return servers, err
}

// UpdateServer updates a server
func (r *Repository) UpdateServer(ctx context.Context, server *models.Server) error {
	return r.db.WithContext(ctx).Save(server).Error
}

// UpdateServerStatus updates only the server status
func (r *Repository) UpdateServerStatus(ctx context.Context, id string, status enums.ServerStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.Server{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateServerProgress updates server provisioning progress
func (r *Repository) UpdateServerProgress(ctx context.Context, id string, progress int, step string) error {
	return r.db.WithContext(ctx).
		Model(&models.Server{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"progress":      progress,
			"progress_step": step,
		}).Error
}

// UpdateServerFields updates specific fields on a server
func (r *Repository) UpdateServerFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.Server{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// ArchiveServer archives a server
func (r *Repository) ArchiveServer(ctx context.Context, id string) error {
	now := time.Now()

	return r.db.WithContext(ctx).
		Model(&models.Server{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"archived_at": now,
			"status":      enums.ServerStatusArchived,
		}).Error
}

// UnarchiveServer unarchives a server
func (r *Repository) UnarchiveServer(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&models.Server{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"archived_at": nil,
			"status":      enums.ServerStatusStopped,
		}).Error
}

// DeleteServer deletes a server
func (r *Repository) DeleteServer(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Server{}, "id = ?", id).Error
}

// CountServersByTeam counts the total number of servers for a team
func (r *Repository) CountServersByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Server{}).
		Where("team_id = ?", teamID).
		Count(&count).Error

	return count, err
}

// ServerHasLaunchAgent checks if a server has the Launch Agent installed
func (r *Repository) ServerHasLaunchAgent(ctx context.Context, serverID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("server_id = ? AND type = ?", serverID, enums.ServiceTypeLaunchAgent).
		Count(&count).Error

	return count > 0, err
}
