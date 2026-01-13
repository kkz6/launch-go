package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CreateCron creates a new cron job
func (r *Repository) CreateCron(ctx context.Context, cron *models.Cron) error {
	return create(r, ctx, cron)
}

// FindCronByID finds a cron job by ID
func (r *Repository) FindCronByID(ctx context.Context, id string) (*models.Cron, error) {
	return findByID[models.Cron](r, ctx, id, ErrCronNotFound)
}

// FindCronByIDWithServer finds a cron job by ID with the Server relation preloaded
func (r *Repository) FindCronByIDWithServer(ctx context.Context, id string) (*models.Cron, error) {
	return findByIDWithPreload[models.Cron](r, ctx, id, "Server", ErrCronNotFound)
}

// FindCronByIDAndServer finds a cron job by ID and server ID
func (r *Repository) FindCronByIDAndServer(ctx context.Context, id, serverID string) (*models.Cron, error) {
	return findByIDAndServer[models.Cron](r, ctx, id, serverID, ErrCronNotFound)
}

// FindCronsByServer finds all cron jobs for a server
func (r *Repository) FindCronsByServer(ctx context.Context, serverID string) ([]models.Cron, error) {
	return findByServer[models.Cron](r, ctx, serverID)
}

// FindVisibleCronsByServer finds all visible (non-hidden) cron jobs for a server
func (r *Repository) FindVisibleCronsByServer(ctx context.Context, serverID string) ([]models.Cron, error) {
	var crons []models.Cron
	err := r.db.WithContext(ctx).
		Where("server_id = ? AND hidden = ?", serverID, false).
		Order("created_at DESC").
		Find(&crons).Error
	return crons, err
}

// UpdateCron updates a cron job
func (r *Repository) UpdateCron(ctx context.Context, cron *models.Cron) error {
	return update(r, ctx, cron)
}

// MarkCronInstalled marks a cron job as installed
func (r *Repository) MarkCronInstalled(ctx context.Context, id string) error {
	return markAsInstalled[models.Cron](r, ctx, id)
}

// DeleteCron deletes a cron job
func (r *Repository) DeleteCron(ctx context.Context, id string) error {
	return deleteByID[models.Cron](r, ctx, id)
}
