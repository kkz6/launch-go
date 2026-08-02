package contracts

import (
	"context"
	"time"

	dbmodels "github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	"gorm.io/gorm"
)

// ServerRepository defines the interface for server database operations
type ServerRepository interface {
	Create(ctx context.Context, server *models.Server) error
	FindByID(ctx context.Context, id string) (*models.Server, error)
	FindByIDAndTeam(ctx context.Context, id, teamID string, preloads ...string) (*models.Server, error)
	FindWithRelations(ctx context.Context, id, teamID string) (*models.Server, error)
	FindAllByTeam(ctx context.Context, teamID string) ([]models.Server, error)
	FindAllByTeamPaginated(ctx context.Context, teamID string, page, perPage int) (*repository.PaginatedResult[models.Server], error)
	FindArchivedByTeam(ctx context.Context, teamID string) ([]models.Server, error)
	Update(ctx context.Context, server *models.Server) error
	UpdateStatus(ctx context.Context, id string, status types.ServerStatus) error
	UpdateProgress(ctx context.Context, id string, progress int, step string) error
	UpdateFields(ctx context.Context, id string, fields map[string]any) error
	UpdateHostKey(ctx context.Context, serverID, hostKey string) error
	Archive(ctx context.Context, id string) error
	Unarchive(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	CountByTeam(ctx context.Context, teamID string) (int64, error)
	HasLaunchAgent(ctx context.Context, serverID string) (bool, error)
}

// ServiceRepository defines the interface for service database operations
type ServiceRepository interface {
	Create(ctx context.Context, service *models.InstalledService) error
	FindByID(ctx context.Context, id string) (*models.InstalledService, error)
	FindByServer(ctx context.Context, serverID string) ([]models.InstalledService, error)
	FindByServerAndType(ctx context.Context, serverID string, serviceType types.ServiceType) ([]models.InstalledService, error)
	FindOneByServerAndType(ctx context.Context, serverID string, serviceType types.ServiceType) (*models.InstalledService, error)
	FindByServerAndSoftware(ctx context.Context, serverID string, software types.Software) (*models.InstalledService, error)
	FindDatabaseService(ctx context.Context, serverID string) (*models.InstalledService, error)
	Update(ctx context.Context, service *models.InstalledService) error
	UpdateStatus(ctx context.Context, id string, status types.ServiceStatus) error
	ClaimPhpPatch(ctx context.Context, id string, previousStatus types.ServiceStatus) (bool, error)
	RestorePhpPatchStatus(ctx context.Context, id string, status types.ServiceStatus) (bool, error)
	UpdateStatusFromProbe(ctx context.Context, id string, status types.ServiceStatus, typeData map[string]any) (bool, error)
	UpdateFields(ctx context.Context, id string, fields map[string]any) error
	UpdateWithTypeData(ctx context.Context, id string, status types.ServiceStatus, typeData map[string]any) error
	Delete(ctx context.Context, id string) error
	SetDefault(ctx context.Context, id string, isDefault bool) error
	UnsetDefaultPhp(ctx context.Context, serverID string) error
	MarkRemovalFailed(ctx context.Context, id string) error
	FindPhpByServerAndVersion(ctx context.Context, serverID, version string) (*models.InstalledService, error)
	AddExtension(ctx context.Context, id, extension string) error
	RemoveExtension(ctx context.Context, id, extension string) error
	SetExtensionStatus(ctx context.Context, id, extension, status string) error
}

// FirewallRuleRepository defines the interface for firewall rule database operations
type FirewallRuleRepository interface {
	Create(ctx context.Context, rule *models.FirewallRule) error
	FindByID(ctx context.Context, id string) (*models.FirewallRule, error)
	FindByIDWithServer(ctx context.Context, id string) (*models.FirewallRule, error)
	FindByIDAndServer(ctx context.Context, id, serverID string) (*models.FirewallRule, error)
	FindByServer(ctx context.Context, serverID string) ([]models.FirewallRule, error)
	Update(ctx context.Context, rule *models.FirewallRule) error
	MarkAsInstalled(ctx context.Context, id string) error
	MarkAsFailed(ctx context.Context, id string) error
	MarkAsUninstalling(ctx context.Context, id string) error
	MarkUninstallationFailed(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

// CronRepository defines the interface for cron job database operations
type CronRepository interface {
	Create(ctx context.Context, cron *models.Cron) error
	FindByID(ctx context.Context, id string) (*models.Cron, error)
	FindByIDWithServer(ctx context.Context, id string) (*models.Cron, error)
	FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Cron, error)
	FindByServer(ctx context.Context, serverID string) ([]models.Cron, error)
	FindVisibleByServer(ctx context.Context, serverID string) ([]models.Cron, error)
	Update(ctx context.Context, cron *models.Cron) error
	MarkAsInstalled(ctx context.Context, id string) error
	MarkInstallationFailed(ctx context.Context, id string) error
	MarkAsUninstalling(ctx context.Context, id string) error
	MarkUninstallationFailed(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	CountBySite(ctx context.Context, siteID string) (int64, error)
}

// DaemonRepository defines the interface for daemon database operations
type DaemonRepository interface {
	Create(ctx context.Context, daemon *models.Daemon) error
	FindByID(ctx context.Context, id string) (*models.Daemon, error)
	FindByIDWithServer(ctx context.Context, id string) (*models.Daemon, error)
	FindByIDAndServer(ctx context.Context, id, serverID string) (*models.Daemon, error)
	FindByServer(ctx context.Context, serverID string) ([]models.Daemon, error)
	Update(ctx context.Context, daemon *models.Daemon) error
	UpdateStatus(ctx context.Context, id string, running bool) error
	UpdateLastStatusCheckByServer(ctx context.Context, serverID string, t time.Time) error
	MarkAsInstalled(ctx context.Context, id string) error
	MarkInstallationFailed(ctx context.Context, id string) error
	MarkAsUninstalling(ctx context.Context, id string) error
	MarkUninstallationFailed(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

// SSHKeyRepository defines the interface for SSH key database operations
type SSHKeyRepository interface {
	Create(ctx context.Context, key *models.SSHKey) error
	FindByID(ctx context.Context, id string) (*models.SSHKey, error)
	FindByTeam(ctx context.Context, teamID string) ([]models.SSHKey, error)
	FindByServer(ctx context.Context, serverID string) ([]models.SSHKey, error)
	FindGlobal(ctx context.Context) ([]models.SSHKey, error)
	FindGlobalByTeam(ctx context.Context, teamID string) ([]models.SSHKey, error)
	Update(ctx context.Context, key *models.SSHKey) error
	Delete(ctx context.Context, id string) error
	AttachToServer(ctx context.Context, serverID, sshKeyID string) error
	DetachFromServer(ctx context.Context, serverID, sshKeyID string) error
	IsAttachedToServer(ctx context.Context, serverID, sshKeyID string) (bool, error)
	DeleteNonGlobalByServer(ctx context.Context, serverID string) error
}

// TaskRepository defines the interface for task database operations
type TaskRepository interface {
	Create(ctx context.Context, task *models.Task) error
	FindByID(ctx context.Context, id string) (*models.Task, error)
	FindByServer(ctx context.Context, serverID string, limit int) ([]models.Task, error)
	FindLatestByServer(ctx context.Context, serverID string) (*models.Task, error)
	Update(ctx context.Context, task *models.Task) error
	UpdateOutput(ctx context.Context, taskID string, output string) error
	UpdateFields(ctx context.Context, taskID string, fields map[string]any) error
}

// MetricRepository defines the interface for metric database operations
type MetricRepository interface {
	Create(ctx context.Context, metric *models.Metric) error
	FindByServer(ctx context.Context, serverID string, from, to *time.Time, limit int) ([]models.Metric, error)
	FindLatestByServer(ctx context.Context, serverID string) (*models.Metric, error)
	DeleteOld(ctx context.Context, serverID string, before time.Time) error
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// ServerProviderRepository defines the interface for server provider database operations
type ServerProviderRepository interface {
	Create(ctx context.Context, provider *models.ServerProvider) error
	FindByID(ctx context.Context, id string) (*models.ServerProvider, error)
	FindByTeam(ctx context.Context, teamID string) ([]models.ServerProvider, error)
	Delete(ctx context.Context, id string) error
}

// DatabaseRepository defines the interface for database operations
type DatabaseRepository interface {
	FindByServer(ctx context.Context, serverID string) ([]dbmodels.Database, error)
	FindByID(ctx context.Context, id string) (*dbmodels.Database, error)
	FindByIDWithServer(ctx context.Context, id string) (*dbmodels.Database, error)
	Create(ctx context.Context, db *dbmodels.Database) error
	Delete(ctx context.Context, id string) error
	FindUsersByServer(ctx context.Context, serverID string) ([]dbmodels.DatabaseUser, error)
	FindUserByID(ctx context.Context, id string) (*dbmodels.DatabaseUser, error)
	FindUserByIDWithServer(ctx context.Context, id string) (*dbmodels.DatabaseUser, error)
	CreateUser(ctx context.Context, user *dbmodels.DatabaseUser) error
	DeleteUser(ctx context.Context, id string) error
}

// LoadBalancerUpstreamRepository defines the interface for LB upstream database operations
type LoadBalancerUpstreamRepository interface {
	Create(ctx context.Context, upstream *models.LoadBalancerUpstream) error
	FindByID(ctx context.Context, id string) (*models.LoadBalancerUpstream, error)
	FindByIDWithBackends(ctx context.Context, id string) (*models.LoadBalancerUpstream, error)
	FindByServerID(ctx context.Context, serverID string) ([]models.LoadBalancerUpstream, error)
	FindByServerIDAndAddress(ctx context.Context, serverID, address string) (*models.LoadBalancerUpstream, error)
	CountByServerID(ctx context.Context, serverID string) (int64, error)
	Update(ctx context.Context, id string, updates map[string]any) error
	Delete(ctx context.Context, id string) error
}

// LoadBalancerBackendRepository defines the interface for LB backend database operations
type LoadBalancerBackendRepository interface {
	Create(ctx context.Context, backend *models.LoadBalancerBackend) error
	FindByID(ctx context.Context, id string) (*models.LoadBalancerBackend, error)
	FindByIDWithRelations(ctx context.Context, id string) (*models.LoadBalancerBackend, error)
	FindByUpstreamID(ctx context.Context, upstreamID string) ([]models.LoadBalancerBackend, error)
	FindBySiteID(ctx context.Context, siteID string) ([]models.LoadBalancerBackend, error)
	FindByUpstreamAndSite(ctx context.Context, upstreamID, siteID string) (*models.LoadBalancerBackend, error)
	CountByUpstreamID(ctx context.Context, upstreamID string) (int64, error)
	Update(ctx context.Context, id string, updates map[string]any) error
	Delete(ctx context.Context, id string) error
	DeleteByUpstreamID(ctx context.Context, upstreamID string) error
}

// RepositoryRegistry provides access to all repositories
type RepositoryRegistry interface {
	Server() ServerRepository
	Service() ServiceRepository
	FirewallRule() FirewallRuleRepository
	Cron() CronRepository
	Daemon() DaemonRepository
	SSHKey() SSHKeyRepository
	Task() TaskRepository
	Metric() MetricRepository
	ServerProvider() ServerProviderRepository
	Database() DatabaseRepository
	LoadBalancerUpstream() LoadBalancerUpstreamRepository
	LoadBalancerBackend() LoadBalancerBackendRepository
	DB() *gorm.DB // Returns the underlying database connection for activity logging
}
