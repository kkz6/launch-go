package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// SiteRepository handles database operations for sites.
// Embeds repository.Installable[T] for CRUD + installation status operations.
type SiteRepository struct {
	repository.Installable[models.Site]
}

// NewSiteRepository creates a new site repository
func NewSiteRepository(db *gorm.DB) *SiteRepository {
	return &SiteRepository{
		Installable: repository.NewInstallable[models.Site](db),
	}
}

// FindByID finds a site by ID with custom error.
func (r *SiteRepository) FindByID(ctx context.Context, id string) (*models.Site, error) {
	site, err := r.Installable.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrSiteNotFound
		}
		return nil, err
	}
	return site, nil
}

// FindByIDWithDeployments finds a site by ID with its deployments
func (r *SiteRepository) FindByIDWithDeployments(ctx context.Context, id string) (*models.Site, error) {
	var site models.Site
	err := r.DB.WithContext(ctx).
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

// FindByIDAndServer finds a site by ID and server ID with custom error.
func (r *SiteRepository) FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Site, error) {
	site, err := r.Installable.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrSiteNotFound
		}
		return nil, err
	}
	return site, nil
}

// FindByIDAndTeam finds a site by ID and team ID with custom error.
func (r *SiteRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Site, error) {
	var site models.Site
	err := r.DB.WithContext(ctx).
		First(&site, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSiteNotFound
		}
		return nil, err
	}
	return &site, nil
}

// FindByIDAndServerAndTeam finds a site by ID, server ID, and team ID.
func (r *SiteRepository) FindByIDAndServerAndTeam(ctx context.Context, id, serverID, teamID string) (*models.Site, error) {
	var site models.Site
	err := r.DB.WithContext(ctx).
		First(&site, "id = ? AND server_id = ? AND team_id = ?", id, serverID, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSiteNotFound
		}
		return nil, err
	}
	return &site, nil
}

// FindAllByTeam finds all sites for a team
func (r *SiteRepository) FindAllByTeam(ctx context.Context, teamID string) ([]models.Site, error) {
	var sites []models.Site
	err := r.DB.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&sites).Error
	return sites, err
}

// FindByServerAndTeam finds sites by server ID and team ID
func (r *SiteRepository) FindByServerAndTeam(ctx context.Context, serverID, teamID string) ([]models.Site, error) {
	var sites []models.Site
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND team_id = ?", serverID, teamID).
		Order("created_at DESC").
		Find(&sites).Error
	return sites, err
}

// CountByTeam counts all sites for a team
func (r *SiteRepository) CountByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Site{}).
		Where("team_id = ?", teamID).
		Count(&count).Error
	return count, err
}

// FindByServerWithLatestDeployment finds sites with their latest deployment
func (r *SiteRepository) FindByServerWithLatestDeployment(ctx context.Context, serverID string) ([]models.Site, error) {
	var sites []models.Site
	err := r.DB.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&sites).Error
	if err != nil {
		return nil, err
	}

	// Load latest deployment for each site
	for i := range sites {
		var deployment models.Deployment
		if err := r.DB.WithContext(ctx).
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
	err := r.DB.WithContext(ctx).
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
	err := r.DB.WithContext(ctx).
		Where("repository_branch = ? AND auto_deployment = ?", branch, true).
		Find(&sites).Error

	return sites, err
}

// Note: The following methods are inherited from repository.Installable[T]:
// From Base[T]:
// - Create(ctx, entity) error
// - FindByServer(ctx, serverID) ([]T, error)
// - Update(ctx, entity) error
// - UpdateFields(ctx, id, fields) error
// - Delete(ctx, id) error
// - Exists(ctx, id) (bool, error)
// - Count(ctx) (int64, error)
// - CountByServer(ctx, serverID) (int64, error)
// - Transaction(ctx, fn) error
// - Query(ctx) *gorm.DB
// - WithPreload(ctx, relations...) *gorm.DB
// From Installable[T]:
// - MarkAsInstalled(ctx, id) error
// - MarkAsFailed(ctx, id) error
// - MarkAsUninstalling(ctx, id) error
// - MarkUninstallationFailed(ctx, id) error
// - FindInstalled(ctx, serverID) ([]T, error)
// - FindPending(ctx, serverID) ([]T, error)
// - FindFailed(ctx, serverID) ([]T, error)
// - FindUninstalling(ctx, serverID) ([]T, error)
