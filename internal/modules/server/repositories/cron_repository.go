package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CronRepository handles cron job database operations
type CronRepository struct {
	BaseRepository
}

// NewCronRepository creates a new CronRepository instance
func NewCronRepository(db *gorm.DB) *CronRepository {
	return &CronRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new cron job
func (r *CronRepository) Create(ctx context.Context, cron *models.Cron) error {
	return r.DB().WithContext(ctx).Create(cron).Error
}

// FindByID finds a cron job by ID
func (r *CronRepository) FindByID(ctx context.Context, id string) (*models.Cron, error) {
	var cron models.Cron
	err := r.DB().WithContext(ctx).First(&cron, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCronNotFound
		}
		return nil, err
	}
	return &cron, nil
}

// FindByIDWithServer finds a cron job by ID with the Server relation preloaded
func (r *CronRepository) FindByIDWithServer(ctx context.Context, id string) (*models.Cron, error) {
	var cron models.Cron
	err := r.DB().WithContext(ctx).Preload("Server").First(&cron, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCronNotFound
		}
		return nil, err
	}
	return &cron, nil
}

// FindByIDAndServer finds a cron job by ID and server ID
func (r *CronRepository) FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Cron, error) {
	var cron models.Cron
	err := r.DB().WithContext(ctx).First(&cron, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCronNotFound
		}
		return nil, err
	}
	return &cron, nil
}

// FindByServer finds all cron jobs for a server
func (r *CronRepository) FindByServer(ctx context.Context, serverID string) ([]models.Cron, error) {
	var crons []models.Cron
	err := r.DB().WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&crons).Error
	return crons, err
}

// FindVisibleByServer finds all visible (non-hidden) cron jobs for a server
func (r *CronRepository) FindVisibleByServer(ctx context.Context, serverID string) ([]models.Cron, error) {
	var crons []models.Cron
	err := r.DB().WithContext(ctx).
		Where("server_id = ? AND hidden = ?", serverID, false).
		Order("created_at DESC").
		Find(&crons).Error
	return crons, err
}

// Update updates a cron job
func (r *CronRepository) Update(ctx context.Context, cron *models.Cron) error {
	return r.DB().WithContext(ctx).Save(cron).Error
}

// MarkInstalled marks a cron job as installed
func (r *CronRepository) MarkInstalled(ctx context.Context, id string) error {
	now := time.Now()
	return r.DB().WithContext(ctx).
		Model(&models.Cron{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"installed_at":           now,
			"installation_failed_at": nil,
		}).Error
}

// Delete deletes a cron job
func (r *CronRepository) Delete(ctx context.Context, id string) error {
	return r.DB().WithContext(ctx).Delete(&models.Cron{}, "id = ?", id).Error
}
