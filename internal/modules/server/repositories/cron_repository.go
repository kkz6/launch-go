package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CreateCron creates a new cron job
func (r *Repository) CreateCron(ctx context.Context, cron *models.Cron) error {
	return r.db.WithContext(ctx).Create(cron).Error
}

// FindCronByID finds a cron job by ID
func (r *Repository) FindCronByID(ctx context.Context, id string) (*models.Cron, error) {
	var cron models.Cron
	err := r.db.WithContext(ctx).First(&cron, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCronNotFound
		}

		return nil, err
	}

	return &cron, nil
}

// FindCronByIDAndServer finds a cron job by ID and server ID
func (r *Repository) FindCronByIDAndServer(ctx context.Context, id, serverID string) (*models.Cron, error) {
	var cron models.Cron
	err := r.db.WithContext(ctx).
		First(&cron, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCronNotFound
		}

		return nil, err
	}

	return &cron, nil
}

// FindCronsByServer finds all cron jobs for a server
func (r *Repository) FindCronsByServer(ctx context.Context, serverID string) ([]models.Cron, error) {
	var crons []models.Cron
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&crons).Error

	return crons, err
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
	return r.db.WithContext(ctx).Save(cron).Error
}

// MarkCronInstalled marks a cron job as installed
func (r *Repository) MarkCronInstalled(ctx context.Context, id string) error {
	now := time.Now()

	return r.db.WithContext(ctx).
		Model(&models.Cron{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"installed_at":           now,
			"installation_failed_at": nil,
		}).Error
}

// DeleteCron deletes a cron job
func (r *Repository) DeleteCron(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Cron{}, "id = ?", id).Error
}
