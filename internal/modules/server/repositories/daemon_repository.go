package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DaemonRepository handles daemon database operations
type DaemonRepository struct {
	repository.Installable[models.Daemon]
}

// NewDaemonRepository creates a new DaemonRepository instance
func NewDaemonRepository(db *gorm.DB) *DaemonRepository {
	return &DaemonRepository{
		Installable: repository.NewInstallable[models.Daemon](db),
	}
}

// FindByID finds a daemon by ID (override to return specific error)
func (r *DaemonRepository) FindByID(ctx context.Context, id string) (*models.Daemon, error) {
	daemon, err := r.Installable.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return daemon, nil
}

// FindByIDWithServer finds a daemon by ID with the Server relation preloaded (override to return specific error)
func (r *DaemonRepository) FindByIDWithServer(ctx context.Context, id string) (*models.Daemon, error) {
	daemon, err := r.Installable.FindByIDWithServer(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return daemon, nil
}

// FindByIDAndServer finds a daemon by ID and server ID (override to return specific error)
func (r *DaemonRepository) FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Daemon, error) {
	daemon, err := r.Installable.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return daemon, nil
}

// UpdateStatus updates a daemon's running status
func (r *DaemonRepository) UpdateStatus(ctx context.Context, id string, running bool) error {
	now := time.Now()
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"running":           running,
		"last_status_check": now,
	})
}

// UpdateLastStatusCheckByServer batch-updates last_status_check for all daemons on a server
func (r *DaemonRepository) UpdateLastStatusCheckByServer(ctx context.Context, serverID string, t time.Time) error {
	return r.DB.WithContext(ctx).
		Model(&models.Daemon{}).
		Where("server_id = ?", serverID).
		Update("last_status_check", t).Error
}

// Note: MarkAsInstalled, MarkInstallationFailed and MarkUninstallationFailed are inherited from repository.Installable
