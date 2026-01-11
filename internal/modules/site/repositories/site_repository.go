package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

// SiteRepository handles database operations for sites
type SiteRepository struct {
	*BaseRepository
}

// NewSiteRepository creates a new site repository
func NewSiteRepository(db *gorm.DB) *SiteRepository {
	return &SiteRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new site
func (r *SiteRepository) Create(ctx context.Context, site *models.Site) error {
	return r.db.WithContext(ctx).Create(site).Error
}

// FindByID finds a site by ID
func (r *SiteRepository) FindByID(ctx context.Context, id string) (*models.Site, error) {
	var site models.Site
	err := r.db.WithContext(ctx).First(&site, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSiteNotFound
		}

		return nil, err
	}

	return &site, nil
}

// FindByIDWithDeployments finds a site by ID with its deployments
func (r *SiteRepository) FindByIDWithDeployments(ctx context.Context, id string) (*models.Site, error) {
	var site models.Site
	err := r.db.WithContext(ctx).
		Preload("Deployments", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(10)
		}).
		First(&site, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSiteNotFound
		}

		return nil, err
	}

	return &site, nil
}

// FindByIDAndServer finds a site by ID and server ID
func (r *SiteRepository) FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Site, error) {
	var site models.Site
	err := r.db.WithContext(ctx).
		First(&site, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSiteNotFound
		}

		return nil, err
	}

	return &site, nil
}

// FindByServer finds all sites for a server
func (r *SiteRepository) FindByServer(ctx context.Context, serverID string) ([]models.Site, error) {
	var sites []models.Site
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&sites).Error

	return sites, err
}

// FindByServerWithLatestDeployment finds sites with their latest deployment
func (r *SiteRepository) FindByServerWithLatestDeployment(ctx context.Context, serverID string) ([]models.Site, error) {
	var sites []models.Site
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&sites).Error
	if err != nil {
		return nil, err
	}

	// Load latest deployment for each site
	for i := range sites {
		var deployment models.Deployment
		if err := r.db.WithContext(ctx).
			Where("site_id = ?", sites[i].ID).
			Order("created_at DESC").
			First(&deployment).Error; err == nil {
			sites[i].LatestDeployment = &deployment
		}
	}

	return sites, nil
}

// FindByAddress finds a site by address and server ID
func (r *SiteRepository) FindByAddress(ctx context.Context, address, serverID string) (*models.Site, error) {
	var site models.Site
	err := r.db.WithContext(ctx).
		First(&site, "address = ? AND server_id = ?", address, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSiteNotFound
		}

		return nil, err
	}

	return &site, nil
}

// FindByRepositoryAndBranch finds sites with auto-deployment enabled for a repository and branch
func (r *SiteRepository) FindByRepositoryAndBranch(ctx context.Context, repository, branch string) ([]models.Site, error) {
	var sites []models.Site
	err := r.db.WithContext(ctx).
		Where("repository_branch = ? AND auto_deployment = ?", branch, true).
		Find(&sites).Error

	return sites, err
}

// Update updates a site
func (r *SiteRepository) Update(ctx context.Context, site *models.Site) error {
	return r.db.WithContext(ctx).Save(site).Error
}

// UpdateFields updates specific fields of a site
func (r *SiteRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.Site{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete deletes a site
func (r *SiteRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Site{}, "id = ?", id).Error
}
