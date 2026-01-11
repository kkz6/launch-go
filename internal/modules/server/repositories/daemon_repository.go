package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CreateDaemon creates a new daemon
func (r *Repository) CreateDaemon(ctx context.Context, daemon *models.Daemon) error {
	return r.db.WithContext(ctx).Create(daemon).Error
}

// FindDaemonByID finds a daemon by ID
func (r *Repository) FindDaemonByID(ctx context.Context, id string) (*models.Daemon, error) {
	var daemon models.Daemon
	err := r.db.WithContext(ctx).First(&daemon, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDaemonNotFound
		}

		return nil, err
	}

	return &daemon, nil
}

// FindDaemonByIDWithServer finds a daemon by ID with the Server relation preloaded
func (r *Repository) FindDaemonByIDWithServer(ctx context.Context, id string) (*models.Daemon, error) {
	var daemon models.Daemon
	err := r.db.WithContext(ctx).Preload("Server").First(&daemon, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDaemonNotFound
		}

		return nil, err
	}

	return &daemon, nil
}

// FindDaemonByIDAndServer finds a daemon by ID and server ID
func (r *Repository) FindDaemonByIDAndServer(ctx context.Context, id, serverID string) (*models.Daemon, error) {
	var daemon models.Daemon
	err := r.db.WithContext(ctx).
		First(&daemon, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDaemonNotFound
		}

		return nil, err
	}

	return &daemon, nil
}

// FindDaemonsByServer finds all daemons for a server
func (r *Repository) FindDaemonsByServer(ctx context.Context, serverID string) ([]models.Daemon, error) {
	var daemons []models.Daemon
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&daemons).Error

	return daemons, err
}

// UpdateDaemon updates a daemon
func (r *Repository) UpdateDaemon(ctx context.Context, daemon *models.Daemon) error {
	return r.db.WithContext(ctx).Save(daemon).Error
}

// UpdateDaemonStatus updates a daemon's running status
func (r *Repository) UpdateDaemonStatus(ctx context.Context, id string, running bool) error {
	now := time.Now()

	return r.db.WithContext(ctx).
		Model(&models.Daemon{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"running":           running,
			"last_status_check": now,
		}).Error
}

// MarkDaemonInstalled marks a daemon as installed
func (r *Repository) MarkDaemonInstalled(ctx context.Context, id string) error {
	now := time.Now()

	return r.db.WithContext(ctx).
		Model(&models.Daemon{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"installed_at":           now,
			"installation_failed_at": nil,
		}).Error
}

// DeleteDaemon deletes a daemon
func (r *Repository) DeleteDaemon(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Daemon{}, "id = ?", id).Error
}
