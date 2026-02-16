package repositories

import (
	"context"
	"errors"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/platform/models"
	"github.com/kkz6/launch-go/internal/modules/platform/types"
)

// ServerPlatformUpdateRepository handles per-server update status tracking
type ServerPlatformUpdateRepository struct {
	db *gorm.DB
}

// NewServerPlatformUpdateRepository creates a new ServerPlatformUpdateRepository
func NewServerPlatformUpdateRepository(db *gorm.DB) *ServerPlatformUpdateRepository {
	return &ServerPlatformUpdateRepository{db: db}
}

// FindByID finds a server platform update by ID
func (r *ServerPlatformUpdateRepository) FindByID(ctx context.Context, id string) (*models.ServerPlatformUpdate, error) {
	var update models.ServerPlatformUpdate
	err := r.db.WithContext(ctx).First(&update, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}

	return &update, nil
}

// Create creates a new server platform update
func (r *ServerPlatformUpdateRepository) Create(ctx context.Context, update *models.ServerPlatformUpdate) error {
	return r.db.WithContext(ctx).Create(update).Error
}

// UpdateStatus updates the status of a server platform update
func (r *ServerPlatformUpdateRepository) UpdateStatus(ctx context.Context, id string, status types.ServerUpdateStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.ServerPlatformUpdate{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Update saves changes to a server platform update
func (r *ServerPlatformUpdateRepository) Update(ctx context.Context, update *models.ServerPlatformUpdate) error {
	return r.db.WithContext(ctx).Save(update).Error
}

// FindByServerAndUpdate finds a server platform update by server ID and platform update ID
func (r *ServerPlatformUpdateRepository) FindByServerAndUpdate(ctx context.Context, serverID, platformUpdateID string) (*models.ServerPlatformUpdate, error) {
	var update models.ServerPlatformUpdate
	err := r.db.WithContext(ctx).
		Where("server_id = ? AND platform_update_id = ?", serverID, platformUpdateID).
		First(&update).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}

	return &update, nil
}

// FindByServerAndUpdateForTeam finds a server platform update, verifying the server belongs to the team
func (r *ServerPlatformUpdateRepository) FindByServerAndUpdateForTeam(ctx context.Context, serverID, platformUpdateID, teamID string) (*models.ServerPlatformUpdate, error) {
	var update models.ServerPlatformUpdate
	err := r.db.WithContext(ctx).
		Table("server_platform_updates spu").
		Select("spu.*").
		Joins("JOIN servers s ON s.id = spu.server_id").
		Where("spu.server_id = ? AND spu.platform_update_id = ? AND s.team_id = ? AND s.archived_at IS NULL", serverID, platformUpdateID, teamID).
		First(&update).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}

	return &update, nil
}

// GetServerStatuses returns all server statuses for a given update, filtered by team
func (r *ServerPlatformUpdateRepository) GetServerStatuses(ctx context.Context, platformUpdateID, teamID string) ([]ServerStatusRow, error) {
	var rows []ServerStatusRow
	err := r.db.WithContext(ctx).
		Table("server_platform_updates spu").
		Select("spu.id, spu.server_id, spu.status, spu.task_id, spu.error_message, spu.completed_at, s.name as server_name").
		Joins("JOIN servers s ON s.id = spu.server_id").
		Where("spu.platform_update_id = ? AND s.team_id = ? AND s.archived_at IS NULL", platformUpdateID, teamID).
		Order("s.name ASC").
		Scan(&rows).Error

	return rows, err
}

// ServerStatusRow holds the joined result of server platform update + server name
type ServerStatusRow struct {
	ID           string                   `json:"id"`
	ServerID     string                   `json:"server_id"`
	Status       types.ServerUpdateStatus `json:"status"`
	TaskID       *string                  `json:"task_id,omitempty"`
	ErrorMessage *string                  `json:"error_message,omitempty"`
	CompletedAt  *string                  `json:"completed_at,omitempty"`
	ServerName   string                   `json:"server_name"`
}

// FindPendingForTeam returns platform updates that have at least one pending server for the team
// and haven't been dismissed by the user
func (r *ServerPlatformUpdateRepository) FindPendingForTeam(ctx context.Context, teamID, userID string) ([]models.PlatformUpdate, error) {
	var updates []models.PlatformUpdate
	err := r.db.WithContext(ctx).
		Distinct("platform_updates.*").
		Table("platform_updates").
		Joins("JOIN server_platform_updates spu ON spu.platform_update_id = platform_updates.id").
		Joins("JOIN servers s ON s.id = spu.server_id").
		Where("s.team_id = ? AND s.archived_at IS NULL", teamID).
		Where("spu.status = ?", types.UpdateStatusPending).
		Where("NOT EXISTS (SELECT 1 FROM platform_update_dismissals d WHERE d.platform_update_id = platform_updates.id AND d.user_id = ?)", userID).
		Order("platform_updates.created_at DESC").
		Find(&updates).Error

	return updates, err
}

// CountByStatus returns the count of server updates grouped by status for a given update and team
func (r *ServerPlatformUpdateRepository) CountByStatus(ctx context.Context, platformUpdateID, teamID string) (map[types.ServerUpdateStatus]int64, error) {
	var results []struct {
		Status types.ServerUpdateStatus
		Count  int64
	}

	err := r.db.WithContext(ctx).
		Table("server_platform_updates spu").
		Select("spu.status, COUNT(*) as count").
		Joins("JOIN servers s ON s.id = spu.server_id").
		Where("spu.platform_update_id = ? AND s.team_id = ? AND s.archived_at IS NULL", platformUpdateID, teamID).
		Group("spu.status").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	counts := make(map[types.ServerUpdateStatus]int64)
	for _, r := range results {
		counts[r.Status] = r.Count
	}

	return counts, nil
}
