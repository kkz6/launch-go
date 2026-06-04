package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
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
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return deployment, nil
}

// FindByIDAndSite finds a deployment by ID and site ID
func (r *DeploymentRepository) FindByIDAndSite(ctx context.Context, id, siteID string) (*models.Deployment, error) {
	return repository.FindOne[models.Deployment](ctx, r.DB,
		repository.WithID(id),
		repository.WithSiteID(siteID),
	)
}

// FindBySite finds all deployments for a site
func (r *DeploymentRepository) FindBySite(ctx context.Context, siteID string) ([]models.Deployment, error) {
	return repository.FindAll[models.Deployment](ctx, r.DB,
		repository.WithSiteID(siteID),
		repository.OrderByCreatedDesc(),
	)
}

// FindLatestBySiteIDs finds the latest deployment for each of the given site IDs.
// Returns a map keyed by site ID.
func (r *DeploymentRepository) FindLatestBySiteIDs(ctx context.Context, siteIDs []string) (map[string]*models.Deployment, error) {
	if len(siteIDs) == 0 {
		return make(map[string]*models.Deployment), nil
	}

	// Build subquery for max created_at per site_id
	subquery := r.DB.Model(&models.Deployment{}).
		Select("site_id, MAX(created_at) as max_created_at").
		Where("site_id IN ?", siteIDs).
		Group("site_id")

	var deployments []models.Deployment
	err := r.DB.WithContext(ctx).
		Joins("INNER JOIN (?) AS latest ON deployments.site_id = latest.site_id AND deployments.created_at = latest.max_created_at", subquery).
		Where("deployments.site_id IN ?", siteIDs).
		Find(&deployments).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]*models.Deployment, len(deployments))
	for i := range deployments {
		result[deployments[i].SiteID] = &deployments[i]
	}

	return result, nil
}

// FindLatestBySite finds the latest deployment for a site
func (r *DeploymentRepository) FindLatestBySite(ctx context.Context, siteID string) (*models.Deployment, error) {
	return repository.FindOneOrNil[models.Deployment](ctx, r.DB,
		repository.WithSiteID(siteID),
		repository.OrderByCreatedDesc(),
	)
}

// FindActiveBySite finds an active deployment for a site
func (r *DeploymentRepository) FindActiveBySite(ctx context.Context, siteID string) (*models.Deployment, error) {
	var deployment models.Deployment
	err := r.DB.WithContext(ctx).
		Where("site_id = ? AND status IN ?", siteID, []sitetypes.DeploymentStatus{sitetypes.DeploymentStatusPending, sitetypes.DeploymentStatusInstalling}).
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
	return repository.FindAll[models.Deployment](ctx, r.DB,
		repository.WithSiteID(siteID),
		repository.WithStatus(string(sitetypes.DeploymentStatusQueued)),
		repository.OrderByCreatedAsc(),
	)
}

// CountQueuedBySite counts queued deployments for a site
func (r *DeploymentRepository) CountQueuedBySite(ctx context.Context, siteID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Deployment{}).
		Where("site_id = ? AND status = ?", siteID, sitetypes.DeploymentStatusQueued).
		Count(&count).Error

	return count, err
}

// UpdateStatus updates a deployment's status
func (r *DeploymentRepository) UpdateStatus(ctx context.Context, id string, status sitetypes.DeploymentStatus) error {
	return r.DB.WithContext(ctx).
		Model(&models.Deployment{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// CancelQueued cancels all queued deployments for a site
func (r *DeploymentRepository) CancelQueued(ctx context.Context, siteID string) (int64, error) {
	result := r.DB.WithContext(ctx).
		Model(&models.Deployment{}).
		Where("site_id = ? AND status = ?", siteID, sitetypes.DeploymentStatusQueued).
		Update("status", sitetypes.DeploymentStatusFailed)

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
