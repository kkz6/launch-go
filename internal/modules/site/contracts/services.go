package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/models"
)

// SiteService defines the interface for site business logic operations
type SiteService interface {
	List(ctx context.Context, serverID string) ([]models.Site, error)
	Create(ctx context.Context, serverID, userID, username string, req *dto.CreateSiteRequest) (*models.Site, error)
	FindByID(ctx context.Context, id, serverID string) (*models.Site, error)
	Update(ctx context.Context, id, serverID, userID string, req *dto.UpdateSiteRequest) (*models.Site, error)
	Delete(ctx context.Context, id, serverID string) error
	GetDeletionSummary(ctx context.Context, id, serverID string) (*dto.DeletionSummaryResponse, error)
	RegenerateDeployToken(ctx context.Context, id, serverID string) error
}

// DeploymentService defines the interface for deployment business logic operations
type DeploymentService interface {
	Deploy(ctx context.Context, siteID, serverID, userID string) (*models.Deployment, error)
	Rollback(ctx context.Context, siteID, serverID, targetDeploymentID, userID string) (*models.Deployment, error)
	List(ctx context.Context, siteID, serverID string) ([]models.Deployment, error)
	FindByID(ctx context.Context, id, siteID, serverID string) (*models.Deployment, error)
	ProcessNextQueued(ctx context.Context, siteID string) (*models.Deployment, error)
	GetQueuedCount(ctx context.Context, siteID string) (int64, error)
	CancelQueued(ctx context.Context, siteID, serverID string) (int64, error)
	EnableAutoDeployment(ctx context.Context, siteID, serverID string) error
	DisableAutoDeployment(ctx context.Context, siteID, serverID string) error
	BroadcastProgress(siteID, deploymentID, status, message string)
}

// SSLService defines the interface for SSL/TLS business logic operations
type SSLService interface {
	UpdateSSL(ctx context.Context, siteID, serverID, userID string, req *dto.UpdateSSLRequest) error
	ListCertificates(ctx context.Context, siteID, serverID string) ([]models.Certificate, error)
}

// QueueService defines the interface for queue worker business logic operations
type QueueService interface {
	Create(ctx context.Context, siteID, serverID, userID string, req *dto.CreateQueueRequest) (*models.Queue, error)
	List(ctx context.Context, siteID, serverID string) ([]models.Queue, error)
	Delete(ctx context.Context, queueID, siteID, serverID string) error
	EnableAutoRestart(ctx context.Context, siteID, serverID string) error
	DisableAutoRestart(ctx context.Context, siteID, serverID string) error
}

// CommandService defines the interface for command execution business logic operations
type CommandService interface {
	Create(ctx context.Context, siteID, serverID, userID string, req *dto.CreateCommandRequest) (*models.Command, error)
	List(ctx context.Context, siteID, serverID string) ([]models.Command, error)
}

// RedirectService defines the interface for redirect business logic operations
type RedirectService interface {
	Create(ctx context.Context, siteID, serverID, userID string, req *dto.CreateRedirectRequest) (*models.Redirect, error)
	List(ctx context.Context, siteID, serverID string) ([]models.Redirect, error)
	Delete(ctx context.Context, redirectID, siteID, serverID string) error
}
