package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
)

// SiteRepository defines the interface for site data access operations
type SiteRepository interface {
	Create(ctx context.Context, site *models.Site) error
	FindByID(ctx context.Context, id string) (*models.Site, error)
	FindByIDWithDeployments(ctx context.Context, id string) (*models.Site, error)
	FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Site, error)
	FindByServer(ctx context.Context, serverID string) ([]models.Site, error)
	FindByServerWithLatestDeployment(ctx context.Context, serverID string) ([]models.Site, error)
	FindByAddress(ctx context.Context, address, serverID string) (*models.Site, error)
	FindByRepositoryAndBranch(ctx context.Context, repository, branch string) ([]models.Site, error)
	Update(ctx context.Context, site *models.Site) error
	UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	WithTransaction(ctx context.Context, fn func(tx SiteRepository) error) error
}

// DeploymentRepository defines the interface for deployment data access operations
type DeploymentRepository interface {
	Create(ctx context.Context, deployment *models.Deployment) error
	FindByID(ctx context.Context, id string) (*models.Deployment, error)
	FindByIDAndSite(ctx context.Context, id, siteID string) (*models.Deployment, error)
	FindBySite(ctx context.Context, siteID string) ([]models.Deployment, error)
	FindLatestBySite(ctx context.Context, siteID string) (*models.Deployment, error)
	FindActiveBySite(ctx context.Context, siteID string) (*models.Deployment, error)
	FindQueuedBySite(ctx context.Context, siteID string) ([]models.Deployment, error)
	CountQueuedBySite(ctx context.Context, siteID string) (int64, error)
	Update(ctx context.Context, deployment *models.Deployment) error
	UpdateStatus(ctx context.Context, id string, status enums.DeploymentStatus) error
	CancelQueued(ctx context.Context, siteID string) (int64, error)
}

// CertificateRepository defines the interface for certificate data access operations
type CertificateRepository interface {
	Create(ctx context.Context, cert *models.Certificate) error
	FindByID(ctx context.Context, id string) (*models.Certificate, error)
	FindBySite(ctx context.Context, siteID string) ([]models.Certificate, error)
	FindActiveBySite(ctx context.Context, siteID string) (*models.Certificate, error)
	Update(ctx context.Context, cert *models.Certificate) error
	Delete(ctx context.Context, id string) error
	DeactivateAll(ctx context.Context, siteID string) error
}

// QueueRepository defines the interface for queue data access operations
type QueueRepository interface {
	Create(ctx context.Context, queue *models.Queue) error
	FindByID(ctx context.Context, id string) (*models.Queue, error)
	FindByIDAndSite(ctx context.Context, id, siteID string) (*models.Queue, error)
	FindBySite(ctx context.Context, siteID string) ([]models.Queue, error)
	FindByServer(ctx context.Context, serverID string) ([]models.Queue, error)
	CountBySite(ctx context.Context, siteID string) (int64, error)
	Update(ctx context.Context, queue *models.Queue) error
	Delete(ctx context.Context, id string) error
}

// CommandRepository defines the interface for command data access operations
type CommandRepository interface {
	Create(ctx context.Context, cmd *models.Command) error
	FindByID(ctx context.Context, id string) (*models.Command, error)
	FindBySite(ctx context.Context, siteID string) ([]models.Command, error)
	Update(ctx context.Context, cmd *models.Command) error
	Delete(ctx context.Context, id string) error
}

// RedirectRepository defines the interface for redirect data access operations
type RedirectRepository interface {
	Create(ctx context.Context, redirect *models.Redirect) error
	FindByID(ctx context.Context, id string) (*models.Redirect, error)
	FindBySite(ctx context.Context, siteID string) ([]models.Redirect, error)
	Update(ctx context.Context, redirect *models.Redirect) error
	Delete(ctx context.Context, id string) error
}

// ReleaseRepository defines the interface for release data access operations
type ReleaseRepository interface {
	Create(ctx context.Context, release *models.Release) error
	FindByID(ctx context.Context, id string) (*models.Release, error)
	FindBySite(ctx context.Context, siteID string) ([]models.Release, error)
	CountBySite(ctx context.Context, siteID string) (int64, error)
	Delete(ctx context.Context, id string) error
	DeleteOld(ctx context.Context, siteID string, keepCount int) error
}
