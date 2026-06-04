package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// LoadBalancerBackendRepository handles load balancer backend database operations
type LoadBalancerBackendRepository struct {
	repository.Base[models.LoadBalancerBackend]
}

// NewLoadBalancerBackendRepository creates a new LoadBalancerBackendRepository
func NewLoadBalancerBackendRepository(db *gorm.DB) *LoadBalancerBackendRepository {
	return &LoadBalancerBackendRepository{
		Base: repository.NewBase[models.LoadBalancerBackend](db),
	}
}

// FindByIDWithRelations returns a backend by ID with upstream and server preloaded
func (r *LoadBalancerBackendRepository) FindByIDWithRelations(ctx context.Context, id string) (*models.LoadBalancerBackend, error) {
	return repository.FindOne[models.LoadBalancerBackend](ctx, r.DB,
		repository.WithID(id),
		repository.Preload("Upstream"),
		repository.Preload("Upstream.Server"),
		repository.Preload("Server"),
	)
}

// FindByUpstreamID returns all backends for a specific upstream
func (r *LoadBalancerBackendRepository) FindByUpstreamID(ctx context.Context, upstreamID string) ([]models.LoadBalancerBackend, error) {
	var backends []models.LoadBalancerBackend
	err := r.DB.WithContext(ctx).
		Preload("Server").
		Where("upstream_id = ?", upstreamID).
		Order("created_at ASC").
		Find(&backends).Error

	return backends, err
}

// FindBySiteID returns all backends for a specific site
func (r *LoadBalancerBackendRepository) FindBySiteID(ctx context.Context, siteID string) ([]models.LoadBalancerBackend, error) {
	return repository.FindAll[models.LoadBalancerBackend](ctx, r.DB, repository.WithSiteID(siteID))
}

// CountByUpstreamID returns the number of backends for an upstream
func (r *LoadBalancerBackendRepository) CountByUpstreamID(ctx context.Context, upstreamID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.LoadBalancerBackend{}).
		Where("upstream_id = ?", upstreamID).
		Count(&count).Error

	return count, err
}

// FindByUpstreamAndSite returns a backend for a specific upstream and site combination
func (r *LoadBalancerBackendRepository) FindByUpstreamAndSite(ctx context.Context, upstreamID, siteID string) (*models.LoadBalancerBackend, error) {
	var backend models.LoadBalancerBackend
	err := r.DB.WithContext(ctx).
		Where("upstream_id = ? AND site_id = ?", upstreamID, siteID).
		First(&backend).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &backend, nil
}

// Update updates a backend's fields
func (r *LoadBalancerBackendRepository) Update(ctx context.Context, id string, updates map[string]any) error {
	result := r.DB.WithContext(ctx).
		Model(&models.LoadBalancerBackend{}).
		Where("id = ?", id).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fiberutil.NotFound()
	}

	return nil
}

// Delete deletes a backend by ID
func (r *LoadBalancerBackendRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.LoadBalancerBackend{}, "id = ?", id).Error
}

// DeleteByUpstreamID deletes all backends for an upstream
func (r *LoadBalancerBackendRepository) DeleteByUpstreamID(ctx context.Context, upstreamID string) error {
	return r.DB.WithContext(ctx).Delete(&models.LoadBalancerBackend{}, "upstream_id = ?", upstreamID).Error
}
