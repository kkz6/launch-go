package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// DaemonRepository handles daemon database operations
type DaemonRepository struct {
	BaseRepository
}

// NewDaemonRepository creates a new DaemonRepository instance
func NewDaemonRepository(db *gorm.DB) *DaemonRepository {
	return &DaemonRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new daemon
func (r *DaemonRepository) Create(ctx context.Context, daemon *models.Daemon) error {
	return r.DB().WithContext(ctx).Create(daemon).Error
}

// FindByID finds a daemon by ID
func (r *DaemonRepository) FindByID(ctx context.Context, id string) (*models.Daemon, error) {
	var daemon models.Daemon
	err := r.DB().WithContext(ctx).First(&daemon, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDaemonNotFound
		}
		return nil, err
	}
	return &daemon, nil
}

// FindByIDWithServer finds a daemon by ID with the Server relation preloaded
func (r *DaemonRepository) FindByIDWithServer(ctx context.Context, id string) (*models.Daemon, error) {
	var daemon models.Daemon
	err := r.DB().WithContext(ctx).Preload("Server").First(&daemon, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDaemonNotFound
		}
		return nil, err
	}
	return &daemon, nil
}

// FindByIDAndServer finds a daemon by ID and server ID
func (r *DaemonRepository) FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Daemon, error) {
	var daemon models.Daemon
	err := r.DB().WithContext(ctx).First(&daemon, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDaemonNotFound
		}
		return nil, err
	}
	return &daemon, nil
}

// FindByServer finds all daemons for a server
func (r *DaemonRepository) FindByServer(ctx context.Context, serverID string) ([]models.Daemon, error) {
	var daemons []models.Daemon
	err := r.DB().WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&daemons).Error
	return daemons, err
}

// Update updates a daemon
func (r *DaemonRepository) Update(ctx context.Context, daemon *models.Daemon) error {
	return r.DB().WithContext(ctx).Save(daemon).Error
}

// UpdateStatus updates a daemon's running status
func (r *DaemonRepository) UpdateStatus(ctx context.Context, id string, running bool) error {
	now := time.Now()
	return r.DB().WithContext(ctx).
		Model(&models.Daemon{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"running":           running,
			"last_status_check": now,
		}).Error
}

// MarkInstalled marks a daemon as installed
func (r *DaemonRepository) MarkInstalled(ctx context.Context, id string) error {
	now := time.Now()
	return r.DB().WithContext(ctx).
		Model(&models.Daemon{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"installed_at":           now,
			"installation_failed_at": nil,
		}).Error
}

// Delete deletes a daemon
func (r *DaemonRepository) Delete(ctx context.Context, id string) error {
	return r.DB().WithContext(ctx).Delete(&models.Daemon{}, "id = ?", id).Error
}
