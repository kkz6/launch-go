package repositories

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// SiteRepository handles database operations for sites.
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
			return nil, fiberutil.NotFound()
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
			return nil, fiberutil.NotFound()
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
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return site, nil
}

// FindByIDAndTeam finds a site by ID and team ID with custom error.
func (r *SiteRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Site, error) {
	return repository.FindOne[models.Site](ctx, r.DB,
		repository.WithID(id),
		repository.WithTeamID(teamID),
	)
}

// FindByIDAndServerAndTeam finds a site by ID, server ID, and team ID.
func (r *SiteRepository) FindByIDAndServerAndTeam(ctx context.Context, id, serverID, teamID string) (*models.Site, error) {
	return repository.FindOne[models.Site](ctx, r.DB,
		repository.WithID(id),
		repository.WithServerID(serverID),
		repository.WithTeamID(teamID),
	)
}

// FindAllByTeam finds all sites for a team
func (r *SiteRepository) FindAllByTeam(ctx context.Context, teamID string) ([]models.Site, error) {
	return repository.FindAll[models.Site](ctx, r.DB,
		repository.WithTeamID(teamID),
		repository.OrderByCreatedDesc(),
	)
}

// FindByServerAndTeam finds sites by server ID and team ID
func (r *SiteRepository) FindByServerAndTeam(ctx context.Context, serverID, teamID string) ([]models.Site, error) {
	return repository.FindAll[models.Site](ctx, r.DB,
		repository.WithServerID(serverID),
		repository.WithTeamID(teamID),
		repository.OrderByCreatedDesc(),
	)
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
// Uses a subquery to efficiently load the latest deployment for each site in a single query,
// avoiding the N+1 query problem.
func (r *SiteRepository) FindByServerWithLatestDeployment(ctx context.Context, serverID string) ([]models.Site, error) {
	var sites []models.Site
	err := r.DB.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&sites).Error
	if err != nil {
		return nil, err
	}

	if len(sites) == 0 {
		return sites, nil
	}

	// Extract site IDs for batch query
	siteIDs := make([]string, len(sites))
	siteMap := make(map[string]*models.Site, len(sites))
	for i := range sites {
		siteIDs[i] = sites[i].ID
		siteMap[sites[i].ID] = &sites[i]
	}

	// Subquery to get the max deployment ID for each site
	// Since we use ULIDs (which are time-sortable), MAX(id) gives us the latest deployment
	latestDeploymentSubquery := r.DB.Model(&models.Deployment{}).
		Select("MAX(id)").
		Where("site_id IN ?", siteIDs).
		Group("site_id")

	var deployments []models.Deployment
	err = r.DB.WithContext(ctx).
		Where("id IN (?)", latestDeploymentSubquery).
		Find(&deployments).Error
	if err != nil {
		return nil, err
	}

	// Map deployments back to their sites
	for i := range deployments {
		if site, ok := siteMap[deployments[i].SiteID]; ok {
			site.LatestDeployment = &deployments[i]
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
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &site, nil
}

// FindByRepositoryAndBranch finds sites with auto-deployment enabled for a repository and branch
func (r *SiteRepository) FindByRepositoryAndBranch(ctx context.Context, repoName, branch string) ([]models.Site, error) {
	var sites []models.Site
	err := r.DB.WithContext(ctx).
		Joins("JOIN source_control_repositories ON source_control_repositories.id = sites.source_control_repositories_id").
		Where("source_control_repositories.full_name = ? AND sites.repository_branch = ? AND sites.auto_deployment = ?", repoName, branch, true).
		Find(&sites).Error

	return sites, err
}

// CreateWithActivity creates a site and logs the activity in a single transaction
func (r *SiteRepository) CreateWithActivity(ctx context.Context, site *models.Site, userID string) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(site).Error; err != nil {
			return fmt.Errorf("failed to insert site record: %w", err)
		}

		// Log activity within the same transaction using activity.LogCreated
		_, err := activity.LogCreated(ctx, tx, userID, site, "Site was created")
		if err != nil {
			return fmt.Errorf("failed to log activity: %w", err)
		}
		return nil
	})
}

// HasSitesBySourceControlID checks if any sites reference the given source control ID
func (r *SiteRepository) HasSitesBySourceControlID(ctx context.Context, sourceControlID string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Site{}).
		Where("source_control_id = ?", sourceControlID).
		Limit(1).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SourceControlExists checks if a source control with the given ID exists for a team
func (r *SiteRepository) SourceControlExists(ctx context.Context, sourceControlID, teamID string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Table("source_controls").
		Where("id = ? AND team_id = ?", sourceControlID, teamID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SourceControlRepositoryExists checks if a source control repository with the given ID exists for a team
func (r *SiteRepository) SourceControlRepositoryExists(ctx context.Context, repositoryID, teamID string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Table("source_control_repositories").
		Joins("JOIN source_controls ON source_controls.id = source_control_repositories.source_control_id").
		Where("source_control_repositories.id = ? AND source_controls.team_id = ?", repositoryID, teamID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
