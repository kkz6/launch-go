package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DeploymentRepository handles database operations for deployments.
type DeploymentRepository struct {
	repository.Base[models.Deployment]
}

// NewDeploymentRepository creates a new deployment repository
func NewDeploymentRepository(db *gorm.DB) *DeploymentRepository {
	return &DeploymentRepository{
		Base: repository.NewBase[models.Deployment](db),
	}
}

// FindByID finds a deployment by ID with custom error.
func (r *DeploymentRepository) FindByID(ctx context.Context, id string) (*models.Deployment, error) {
	deployment, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrDeploymentNotFound
		}
		return nil, err
	}
	return deployment, nil
}

// FindByIDAndSite finds a deployment by ID and site ID
func (r *DeploymentRepository) FindByIDAndSite(ctx context.Context, id, siteID string) (*models.Deployment, error) {
	var deployment models.Deployment
	err := r.DB.WithContext(ctx).
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
	err := r.DB.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&deployments).Error

	return deployments, err
}

// FindLatestBySite finds the latest deployment for a site
func (r *DeploymentRepository) FindLatestBySite(ctx context.Context, siteID string) (*models.Deployment, error) {
	var deployment models.Deployment
	err := r.DB.WithContext(ctx).
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
	err := r.DB.WithContext(ctx).
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
	err := r.DB.WithContext(ctx).
		Where("site_id = ? AND status = ?", siteID, enums.DeploymentStatusQueued).
		Order("created_at ASC").
		Find(&deployments).Error

	return deployments, err
}

// CountQueuedBySite counts queued deployments for a site
func (r *DeploymentRepository) CountQueuedBySite(ctx context.Context, siteID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Deployment{}).
		Where("site_id = ? AND status = ?", siteID, enums.DeploymentStatusQueued).
		Count(&count).Error

	return count, err
}

// UpdateStatus updates a deployment's status
func (r *DeploymentRepository) UpdateStatus(ctx context.Context, id string, status enums.DeploymentStatus) error {
	return r.DB.WithContext(ctx).
		Model(&models.Deployment{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// CancelQueued cancels all queued deployments for a site
func (r *DeploymentRepository) CancelQueued(ctx context.Context, siteID string) (int64, error) {
	result := r.DB.WithContext(ctx).
		Model(&models.Deployment{}).
		Where("site_id = ? AND status = ?", siteID, enums.DeploymentStatusQueued).
		Update("status", enums.DeploymentStatusFailed)

	return result.RowsAffected, result.Error
}

// DeleteBySite deletes all deployments for a site
func (r *DeploymentRepository) DeleteBySite(ctx context.Context, siteID string) error {
	return r.DB.WithContext(ctx).
		Where("site_id = ?", siteID).
		Delete(&models.Deployment{}).Error
}

// CleanupOldDeployments removes old deployment records beyond the retention limit.
// For zero-downtime deployments, it uses the site's DeploymentReleasesRetention setting.
// For normal deployments, it keeps only the most recent 5 deployments.
// Returns the number of deleted deployments and any error.
func (r *DeploymentRepository) CleanupOldDeployments(ctx context.Context, siteID string, retentionCount int) (int64, error) {
	// Get IDs of deployments to keep (most recent N)
	var deploymentsToKeep []string
	err := r.DB.WithContext(ctx).
		Model(&models.Deployment{}).
		Select("id").
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Limit(retentionCount).
		Pluck("id", &deploymentsToKeep).Error
	if err != nil {
		return 0, err
	}

	// If we have fewer deployments than retention, nothing to delete
	if len(deploymentsToKeep) < retentionCount {
		return 0, nil
	}

	// Delete deployments not in the keep list
	result := r.DB.WithContext(ctx).
		Where("site_id = ? AND id NOT IN ?", siteID, deploymentsToKeep).
		Delete(&models.Deployment{})

	return result.RowsAffected, result.Error
}
