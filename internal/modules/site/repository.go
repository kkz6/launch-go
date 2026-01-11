package site

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrSiteNotFound       = errors.New("site not found")
	ErrDeploymentNotFound = errors.New("deployment not found")
	ErrQueueNotFound      = errors.New("queue not found")
	ErrCertificateNotFound = errors.New("certificate not found")
	ErrRedirectNotFound   = errors.New("redirect not found")
	ErrCommandNotFound    = errors.New("command not found")
	ErrReleaseNotFound    = errors.New("release not found")
)

// Repository handles database operations for sites and related entities
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new site repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Site operations

// Create creates a new site
func (r *Repository) Create(ctx context.Context, site *Site) error {
	return r.db.WithContext(ctx).Create(site).Error
}

// FindByID finds a site by ID
func (r *Repository) FindByID(ctx context.Context, id string) (*Site, error) {
	var site Site
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
func (r *Repository) FindByIDWithDeployments(ctx context.Context, id string) (*Site, error) {
	var site Site
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
func (r *Repository) FindByIDAndServer(ctx context.Context, id, serverID string) (*Site, error) {
	var site Site
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
func (r *Repository) FindByServer(ctx context.Context, serverID string) ([]Site, error) {
	var sites []Site
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&sites).Error
	return sites, err
}

// FindByServerWithLatestDeployment finds sites with their latest deployment
func (r *Repository) FindByServerWithLatestDeployment(ctx context.Context, serverID string) ([]Site, error) {
	var sites []Site
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&sites).Error
	if err != nil {
		return nil, err
	}

	// Load latest deployment for each site
	for i := range sites {
		var deployment Deployment
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
func (r *Repository) FindByAddress(ctx context.Context, address, serverID string) (*Site, error) {
	var site Site
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
func (r *Repository) FindByRepositoryAndBranch(ctx context.Context, repository, branch string) ([]Site, error) {
	var sites []Site
	err := r.db.WithContext(ctx).
		Where("repository_branch = ? AND auto_deployment = ?", branch, true).
		Find(&sites).Error
	return sites, err
}

// Update updates a site
func (r *Repository) Update(ctx context.Context, site *Site) error {
	return r.db.WithContext(ctx).Save(site).Error
}

// UpdateFields updates specific fields of a site
func (r *Repository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&Site{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete deletes a site
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Site{}, "id = ?", id).Error
}

// Deployment operations

// CreateDeployment creates a new deployment
func (r *Repository) CreateDeployment(ctx context.Context, deployment *Deployment) error {
	return r.db.WithContext(ctx).Create(deployment).Error
}

// FindDeploymentByID finds a deployment by ID
func (r *Repository) FindDeploymentByID(ctx context.Context, id string) (*Deployment, error) {
	var deployment Deployment
	err := r.db.WithContext(ctx).First(&deployment, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeploymentNotFound
		}
		return nil, err
	}
	return &deployment, nil
}

// FindDeploymentByIDAndSite finds a deployment by ID and site ID
func (r *Repository) FindDeploymentByIDAndSite(ctx context.Context, id, siteID string) (*Deployment, error) {
	var deployment Deployment
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

// FindDeploymentsBySite finds all deployments for a site
func (r *Repository) FindDeploymentsBySite(ctx context.Context, siteID string) ([]Deployment, error) {
	var deployments []Deployment
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&deployments).Error
	return deployments, err
}

// FindLatestDeploymentBySite finds the latest deployment for a site
func (r *Repository) FindLatestDeploymentBySite(ctx context.Context, siteID string) (*Deployment, error) {
	var deployment Deployment
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

// FindActiveDeploymentBySite finds an active deployment for a site
func (r *Repository) FindActiveDeploymentBySite(ctx context.Context, siteID string) (*Deployment, error) {
	var deployment Deployment
	err := r.db.WithContext(ctx).
		Where("site_id = ? AND status IN ?", siteID, []DeploymentStatus{DeploymentStatusPending, DeploymentStatusInstalling}).
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

// FindQueuedDeploymentsBySite finds queued deployments for a site
func (r *Repository) FindQueuedDeploymentsBySite(ctx context.Context, siteID string) ([]Deployment, error) {
	var deployments []Deployment
	err := r.db.WithContext(ctx).
		Where("site_id = ? AND status = ?", siteID, DeploymentStatusQueued).
		Order("created_at ASC").
		Find(&deployments).Error
	return deployments, err
}

// CountQueuedDeploymentsBySite counts queued deployments for a site
func (r *Repository) CountQueuedDeploymentsBySite(ctx context.Context, siteID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Deployment{}).
		Where("site_id = ? AND status = ?", siteID, DeploymentStatusQueued).
		Count(&count).Error
	return count, err
}

// UpdateDeployment updates a deployment
func (r *Repository) UpdateDeployment(ctx context.Context, deployment *Deployment) error {
	return r.db.WithContext(ctx).Save(deployment).Error
}

// UpdateDeploymentStatus updates a deployment's status
func (r *Repository) UpdateDeploymentStatus(ctx context.Context, id string, status DeploymentStatus) error {
	return r.db.WithContext(ctx).
		Model(&Deployment{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// CancelQueuedDeployments cancels all queued deployments for a site
func (r *Repository) CancelQueuedDeployments(ctx context.Context, siteID string) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&Deployment{}).
		Where("site_id = ? AND status = ?", siteID, DeploymentStatusQueued).
		Update("status", DeploymentStatusFailed)
	return result.RowsAffected, result.Error
}

// Certificate operations

// CreateCertificate creates a new certificate
func (r *Repository) CreateCertificate(ctx context.Context, cert *Certificate) error {
	return r.db.WithContext(ctx).Create(cert).Error
}

// FindCertificateByID finds a certificate by ID
func (r *Repository) FindCertificateByID(ctx context.Context, id string) (*Certificate, error) {
	var cert Certificate
	err := r.db.WithContext(ctx).First(&cert, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCertificateNotFound
		}
		return nil, err
	}
	return &cert, nil
}

// FindCertificatesBySite finds all certificates for a site
func (r *Repository) FindCertificatesBySite(ctx context.Context, siteID string) ([]Certificate, error) {
	var certs []Certificate
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&certs).Error
	return certs, err
}

// FindActiveCertificateBySite finds the active certificate for a site
func (r *Repository) FindActiveCertificateBySite(ctx context.Context, siteID string) (*Certificate, error) {
	var cert Certificate
	err := r.db.WithContext(ctx).
		Where("site_id = ? AND is_active = ?", siteID, true).
		First(&cert).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &cert, nil
}

// UpdateCertificate updates a certificate
func (r *Repository) UpdateCertificate(ctx context.Context, cert *Certificate) error {
	return r.db.WithContext(ctx).Save(cert).Error
}

// DeleteCertificate deletes a certificate
func (r *Repository) DeleteCertificate(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Certificate{}, "id = ?", id).Error
}

// DeactivateAllCertificates deactivates all certificates for a site
func (r *Repository) DeactivateAllCertificates(ctx context.Context, siteID string) error {
	return r.db.WithContext(ctx).
		Model(&Certificate{}).
		Where("site_id = ?", siteID).
		Update("is_active", false).Error
}

// Queue operations

// CreateQueue creates a new queue
func (r *Repository) CreateQueue(ctx context.Context, queue *Queue) error {
	return r.db.WithContext(ctx).Create(queue).Error
}

// FindQueueByID finds a queue by ID
func (r *Repository) FindQueueByID(ctx context.Context, id string) (*Queue, error) {
	var queue Queue
	err := r.db.WithContext(ctx).First(&queue, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQueueNotFound
		}
		return nil, err
	}
	return &queue, nil
}

// FindQueueByIDAndSite finds a queue by ID and site ID
func (r *Repository) FindQueueByIDAndSite(ctx context.Context, id, siteID string) (*Queue, error) {
	var queue Queue
	err := r.db.WithContext(ctx).
		First(&queue, "id = ? AND site_id = ?", id, siteID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQueueNotFound
		}
		return nil, err
	}
	return &queue, nil
}

// FindQueuesBySite finds all queues for a site
func (r *Repository) FindQueuesBySite(ctx context.Context, siteID string) ([]Queue, error) {
	var queues []Queue
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&queues).Error
	return queues, err
}

// FindQueuesByServer finds all queues for a server
func (r *Repository) FindQueuesByServer(ctx context.Context, serverID string) ([]Queue, error) {
	var queues []Queue
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&queues).Error
	return queues, err
}

// CountQueuesBySite counts queues for a site
func (r *Repository) CountQueuesBySite(ctx context.Context, siteID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Queue{}).
		Where("site_id = ?", siteID).
		Count(&count).Error
	return count, err
}

// UpdateQueue updates a queue
func (r *Repository) UpdateQueue(ctx context.Context, queue *Queue) error {
	return r.db.WithContext(ctx).Save(queue).Error
}

// DeleteQueue deletes a queue
func (r *Repository) DeleteQueue(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Queue{}, "id = ?", id).Error
}

// Command operations

// CreateCommand creates a new command
func (r *Repository) CreateCommand(ctx context.Context, cmd *Command) error {
	return r.db.WithContext(ctx).Create(cmd).Error
}

// FindCommandByID finds a command by ID
func (r *Repository) FindCommandByID(ctx context.Context, id string) (*Command, error) {
	var cmd Command
	err := r.db.WithContext(ctx).First(&cmd, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCommandNotFound
		}
		return nil, err
	}
	return &cmd, nil
}

// FindCommandsBySite finds all commands for a site
func (r *Repository) FindCommandsBySite(ctx context.Context, siteID string) ([]Command, error) {
	var cmds []Command
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&cmds).Error
	return cmds, err
}

// UpdateCommand updates a command
func (r *Repository) UpdateCommand(ctx context.Context, cmd *Command) error {
	return r.db.WithContext(ctx).Save(cmd).Error
}

// DeleteCommand deletes a command
func (r *Repository) DeleteCommand(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Command{}, "id = ?", id).Error
}

// Redirect operations

// CreateRedirect creates a new redirect
func (r *Repository) CreateRedirect(ctx context.Context, redirect *Redirect) error {
	return r.db.WithContext(ctx).Create(redirect).Error
}

// FindRedirectByID finds a redirect by ID
func (r *Repository) FindRedirectByID(ctx context.Context, id string) (*Redirect, error) {
	var redirect Redirect
	err := r.db.WithContext(ctx).First(&redirect, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRedirectNotFound
		}
		return nil, err
	}
	return &redirect, nil
}

// FindRedirectsBySite finds all redirects for a site
func (r *Repository) FindRedirectsBySite(ctx context.Context, siteID string) ([]Redirect, error) {
	var redirects []Redirect
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&redirects).Error
	return redirects, err
}

// UpdateRedirect updates a redirect
func (r *Repository) UpdateRedirect(ctx context.Context, redirect *Redirect) error {
	return r.db.WithContext(ctx).Save(redirect).Error
}

// DeleteRedirect deletes a redirect
func (r *Repository) DeleteRedirect(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Redirect{}, "id = ?", id).Error
}

// Release operations

// CreateRelease creates a new release
func (r *Repository) CreateRelease(ctx context.Context, release *Release) error {
	return r.db.WithContext(ctx).Create(release).Error
}

// FindReleaseByID finds a release by ID
func (r *Repository) FindReleaseByID(ctx context.Context, id string) (*Release, error) {
	var release Release
	err := r.db.WithContext(ctx).First(&release, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReleaseNotFound
		}
		return nil, err
	}
	return &release, nil
}

// FindReleasesBySite finds all releases for a site
func (r *Repository) FindReleasesBySite(ctx context.Context, siteID string) ([]Release, error) {
	var releases []Release
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&releases).Error
	return releases, err
}

// CountReleasesBySite counts releases for a site
func (r *Repository) CountReleasesBySite(ctx context.Context, siteID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Release{}).
		Where("site_id = ?", siteID).
		Count(&count).Error
	return count, err
}

// DeleteRelease deletes a release
func (r *Repository) DeleteRelease(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Release{}, "id = ?", id).Error
}

// DeleteOldReleases deletes old releases beyond the retention limit
func (r *Repository) DeleteOldReleases(ctx context.Context, siteID string, keepCount int) error {
	// Get IDs of releases to keep
	var keepIDs []string
	if err := r.db.WithContext(ctx).
		Model(&Release{}).
		Select("id").
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Limit(keepCount).
		Pluck("id", &keepIDs).Error; err != nil {
		return err
	}

	if len(keepIDs) == 0 {
		return nil
	}

	// Delete releases not in the keep list
	return r.db.WithContext(ctx).
		Where("site_id = ? AND id NOT IN ?", siteID, keepIDs).
		Delete(&Release{}).Error
}

// Transaction support

// WithTransaction executes operations within a transaction
func (r *Repository) WithTransaction(ctx context.Context, fn func(tx *Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}
