package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ServerRepository handles server database operations
type ServerRepository struct {
	repository.Base[models.Server]
}

// NewServerRepository creates a new ServerRepository instance
func NewServerRepository(db *gorm.DB) *ServerRepository {
	return &ServerRepository{
		Base: repository.NewBase[models.Server](db),
	}
}

// Create creates a new server
func (r *ServerRepository) Create(ctx context.Context, server *models.Server) error {
	return r.Base.Create(ctx, server)
}

// FindByID finds a server by ID with Services preloaded
func (r *ServerRepository) FindByID(ctx context.Context, id string) (*models.Server, error) {
	var server models.Server
	err := r.DB.WithContext(ctx).
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

// FindByIDAndTeam finds a server by ID and team ID with Services preloaded
func (r *ServerRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Server, error) {
	var server models.Server
	err := r.DB.WithContext(ctx).
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

// FindWithRelations finds a server with all relations
func (r *ServerRepository) FindWithRelations(ctx context.Context, id, teamID string) (*models.Server, error) {
	var server models.Server
	err := r.DB.WithContext(ctx).
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

// FindAllByTeam finds all active (non-archived) servers for a team
func (r *ServerRepository) FindAllByTeam(ctx context.Context, teamID string) ([]models.Server, error) {
	var servers []models.Server
	err := r.DB.WithContext(ctx).
		Select("servers.*, (SELECT COUNT(*) FROM sites WHERE sites.server_id = servers.id) as sites_count").
		Preload("Services").
		Scopes(repository.WithTeamID(teamID), repository.WithActive()).
		Order("created_at DESC").
		Find(&servers).Error
	return servers, err
}

// FindAllByTeamPaginated finds all servers for a team with pagination
func (r *ServerRepository) FindAllByTeamPaginated(ctx context.Context, teamID string, limit, offset int) ([]models.Server, int64, error) {
	var servers []models.Server
	var total int64

	query := r.DB.WithContext(ctx).Model(&models.Server{}).Where("team_id = ?", teamID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.DB.WithContext(ctx).
		Select("servers.*, (SELECT COUNT(*) FROM sites WHERE sites.server_id = servers.id) as sites_count").
		Preload("Services").
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&servers).Error

	return servers, total, err
}

// FindArchivedByTeam finds all archived servers for a team
func (r *ServerRepository) FindArchivedByTeam(ctx context.Context, teamID string) ([]models.Server, error) {
	var servers []models.Server
	err := r.DB.WithContext(ctx).
		Select("servers.*, (SELECT COUNT(*) FROM sites WHERE sites.server_id = servers.id) as sites_count").
		Unscoped().
		Where("team_id = ? AND archived_at IS NOT NULL", teamID).
		Order("archived_at DESC").
		Find(&servers).Error
	return servers, err
}

// Update updates a server
func (r *ServerRepository) Update(ctx context.Context, server *models.Server) error {
	return r.Base.Update(ctx, server)
}

// UpdateStatus updates only the server status
func (r *ServerRepository) UpdateStatus(ctx context.Context, id string, status enums.ServerStatus) error {
	return r.Base.UpdateFields(ctx, id, map[string]interface{}{
		"status": status,
	})
}

// UpdateProgress updates server provisioning progress
func (r *ServerRepository) UpdateProgress(ctx context.Context, id string, progress int, step string) error {
	return r.Base.UpdateFields(ctx, id, map[string]interface{}{
		"progress":      progress,
		"progress_step": step,
	})
}

// UpdateFields updates specific fields on a server
func (r *ServerRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.Base.UpdateFields(ctx, id, fields)
}

// Archive archives a server
func (r *ServerRepository) Archive(ctx context.Context, id string) error {
	now := time.Now()
	return r.Base.UpdateFields(ctx, id, map[string]interface{}{
		"archived_at": now,
		"status":      enums.ServerStatusArchived,
	})
}

// Unarchive unarchives a server
func (r *ServerRepository) Unarchive(ctx context.Context, id string) error {
	return r.Base.UpdateFields(ctx, id, map[string]interface{}{
		"archived_at": nil,
		"status":      enums.ServerStatusStopped,
	})
}

// Delete deletes a server
func (r *ServerRepository) Delete(ctx context.Context, id string) error {
	return r.Base.Delete(ctx, id)
}

// CountByTeam counts the total number of servers for a team
func (r *ServerRepository) CountByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Server{}).
		Where("team_id = ?", teamID).
		Count(&count).Error
	return count, err
}

// HasLaunchAgent checks if a server has the Launch Agent installed
func (r *ServerRepository) HasLaunchAgent(ctx context.Context, serverID string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("server_id = ? AND type = ?", serverID, enums.ServiceTypeLaunchAgent).
		Count(&count).Error
	return count > 0, err
}
