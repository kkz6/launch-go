package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// LoadBalancerUpstreamRepository handles load balancer upstream database operations
type LoadBalancerUpstreamRepository struct {
	repository.Base[models.LoadBalancerUpstream]
}

// NewLoadBalancerUpstreamRepository creates a new LoadBalancerUpstreamRepository
func NewLoadBalancerUpstreamRepository(db *gorm.DB) *LoadBalancerUpstreamRepository {
	return &LoadBalancerUpstreamRepository{
		Base: repository.NewBase[models.LoadBalancerUpstream](db),
	}
}

// FindByIDWithBackends returns an upstream by ID with backends and their servers preloaded
func (r *LoadBalancerUpstreamRepository) FindByIDWithBackends(ctx context.Context, id string) (*models.LoadBalancerUpstream, error) {
	var upstream models.LoadBalancerUpstream
	err := r.DB.WithContext(ctx).
		Preload("Backends").
		Preload("Backends.Server").
		Preload("Server").
		First(&upstream, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}

	return &upstream, nil
}

// FindByServerID returns all upstreams for a load balancer server
func (r *LoadBalancerUpstreamRepository) FindByServerID(ctx context.Context, serverID string) ([]models.LoadBalancerUpstream, error) {
	var upstreams []models.LoadBalancerUpstream
	err := r.DB.WithContext(ctx).
		Preload("Backends").
		Preload("Backends.Server").
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&upstreams).Error

	return upstreams, err
}

// FindByServerIDAndAddress returns an upstream by server and address (for uniqueness check)
func (r *LoadBalancerUpstreamRepository) FindByServerIDAndAddress(ctx context.Context, serverID, address string) (*models.LoadBalancerUpstream, error) {
	var upstream models.LoadBalancerUpstream
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND address = ?", serverID, address).
		First(&upstream).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &upstream, nil
}

// CountByServerID returns the number of upstreams for a server
func (r *LoadBalancerUpstreamRepository) CountByServerID(ctx context.Context, serverID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.LoadBalancerUpstream{}).
		Where("server_id = ?", serverID).
		Count(&count).Error

	return count, err
}

// Update updates an upstream's fields
func (r *LoadBalancerUpstreamRepository) Update(ctx context.Context, id string, updates map[string]any) error {
	result := r.DB.WithContext(ctx).
		Model(&models.LoadBalancerUpstream{}).
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

// Delete deletes an upstream by ID
func (r *LoadBalancerUpstreamRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.LoadBalancerUpstream{}, "id = ?", id).Error
}
