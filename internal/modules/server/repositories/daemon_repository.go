package repositories

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CreateDaemon creates a new daemon
func (r *Repository) CreateDaemon(ctx context.Context, daemon *models.Daemon) error {
	return create(r, ctx, daemon)
}

// FindDaemonByID finds a daemon by ID
func (r *Repository) FindDaemonByID(ctx context.Context, id string) (*models.Daemon, error) {
	return findByID[models.Daemon](r, ctx, id, ErrDaemonNotFound)
}

// FindDaemonByIDWithServer finds a daemon by ID with the Server relation preloaded
func (r *Repository) FindDaemonByIDWithServer(ctx context.Context, id string) (*models.Daemon, error) {
	return findByIDWithPreload[models.Daemon](r, ctx, id, "Server", ErrDaemonNotFound)
}

// FindDaemonByIDAndServer finds a daemon by ID and server ID
func (r *Repository) FindDaemonByIDAndServer(ctx context.Context, id, serverID string) (*models.Daemon, error) {
	return findByIDAndServer[models.Daemon](r, ctx, id, serverID, ErrDaemonNotFound)
}

// FindDaemonsByServer finds all daemons for a server
func (r *Repository) FindDaemonsByServer(ctx context.Context, serverID string) ([]models.Daemon, error) {
	return findByServer[models.Daemon](r, ctx, serverID)
}

// UpdateDaemon updates a daemon
func (r *Repository) UpdateDaemon(ctx context.Context, daemon *models.Daemon) error {
	return update(r, ctx, daemon)
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
	return markAsInstalled[models.Daemon](r, ctx, id)
}

// DeleteDaemon deletes a daemon
func (r *Repository) DeleteDaemon(ctx context.Context, id string) error {
	return deleteByID[models.Daemon](r, ctx, id)
}
