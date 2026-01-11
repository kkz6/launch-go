package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
)

// DeploymentRepository handles database operations for deployments
type DeploymentRepository struct {
	*BaseRepository
}

// NewDeploymentRepository creates a new deployment repository
func NewDeploymentRepository(db *gorm.DB) *DeploymentRepository {
	return &DeploymentRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new deployment
func (r *DeploymentRepository) Create(ctx context.Context, deployment *models.Deployment) error {
	return r.db.WithContext(ctx).Create(deployment).Error
}

// FindByID finds a deployment by ID
func (r *DeploymentRepository) FindByID(ctx context.Context, id string) (*models.Deployment, error) {
	var deployment models.Deployment
	err := r.db.WithContext(ctx).First(&deployment, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeploymentNotFound
		}

		return nil, err
	}

	return &deployment, nil
}

// FindByIDAndSite finds a deployment by ID and site ID
func (r *DeploymentRepository) FindByIDAndSite(ctx context.Context, id, siteID string) (*models.Deployment, error) {
	var deployment models.Deployment
	err := r.db.WithContext(ctx).
		First(&deployment, "id = ? AND site_id = ?", id, siteID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeploymentNotFound
		}

		return nil, err
	}

	return &deployment, nil
}

// FindBySite finds all deployments for a site
func (r *DeploymentRepository) FindBySite(ctx context.Context, siteID string) ([]models.Deployment, error) {
	var deployments []models.Deployment
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&deployments).Error

	return deployments, err
}

// FindLatestBySite finds the latest deployment for a site
func (r *DeploymentRepository) FindLatestBySite(ctx context.Context, siteID string) (*models.Deployment, error) {
	var deployment models.Deployment
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		First(&deployment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &deployment, nil
}

// FindActiveBySite finds an active deployment for a site
func (r *DeploymentRepository) FindActiveBySite(ctx context.Context, siteID string) (*models.Deployment, error) {
	var deployment models.Deployment
	err := r.db.WithContext(ctx).
		Where("site_id = ? AND status IN ?", siteID, []enums.DeploymentStatus{enums.DeploymentStatusPending, enums.DeploymentStatusInstalling}).
		Order("created_at DESC").
		First(&deployment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &deployment, nil
}

// FindQueuedBySite finds queued deployments for a site
func (r *DeploymentRepository) FindQueuedBySite(ctx context.Context, siteID string) ([]models.Deployment, error) {
	var deployments []models.Deployment
	err := r.db.WithContext(ctx).
		Where("site_id = ? AND status = ?", siteID, enums.DeploymentStatusQueued).
		Order("created_at ASC").
		Find(&deployments).Error

	return deployments, err
}

// CountQueuedBySite counts queued deployments for a site
func (r *DeploymentRepository) CountQueuedBySite(ctx context.Context, siteID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Deployment{}).
		Where("site_id = ? AND status = ?", siteID, enums.DeploymentStatusQueued).
		Count(&count).Error

	return count, err
}

// Update updates a deployment
func (r *DeploymentRepository) Update(ctx context.Context, deployment *models.Deployment) error {
	return r.db.WithContext(ctx).Save(deployment).Error
}

// UpdateStatus updates a deployment's status
func (r *DeploymentRepository) UpdateStatus(ctx context.Context, id string, status enums.DeploymentStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.Deployment{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// CancelQueued cancels all queued deployments for a site
func (r *DeploymentRepository) CancelQueued(ctx context.Context, siteID string) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&models.Deployment{}).
		Where("site_id = ? AND status = ?", siteID, enums.DeploymentStatusQueued).
		Update("status", enums.DeploymentStatusFailed)

	return result.RowsAffected, result.Error
}
