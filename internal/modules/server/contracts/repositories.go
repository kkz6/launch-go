package contracts

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// ServerRepository defines the interface for server database operations
type ServerRepository interface {
	CreateServer(ctx context.Context, server *models.Server) error
	FindServerByID(ctx context.Context, id string) (*models.Server, error)
	FindServerByIDAndTeam(ctx context.Context, id, teamID string) (*models.Server, error)
	FindServerWithRelations(ctx context.Context, id, teamID string) (*models.Server, error)
	FindAllServersByTeam(ctx context.Context, teamID string) ([]models.Server, error)
	FindAllServersByTeamPaginated(ctx context.Context, teamID string, limit, offset int) ([]models.Server, int64, error)
	FindArchivedServersByTeam(ctx context.Context, teamID string) ([]models.Server, error)
	UpdateServer(ctx context.Context, server *models.Server) error
	UpdateServerStatus(ctx context.Context, id string, status enums.ServerStatus) error
	UpdateServerProgress(ctx context.Context, id string, progress int, step string) error
	UpdateServerFields(ctx context.Context, id string, fields map[string]interface{}) error
	ArchiveServer(ctx context.Context, id string) error
	UnarchiveServer(ctx context.Context, id string) error
	DeleteServer(ctx context.Context, id string) error
	CountServersByTeam(ctx context.Context, teamID string) (int64, error)
	ServerHasLaunchAgent(ctx context.Context, serverID string) (bool, error)
}

// ServiceRepository defines the interface for service database operations
type ServiceRepository interface {
	CreateService(ctx context.Context, service *models.InstalledService) error
	FindServiceByID(ctx context.Context, id string) (*models.InstalledService, error)
	FindServicesByServer(ctx context.Context, serverID string) ([]models.InstalledService, error)
	FindServiceByServerAndType(ctx context.Context, serverID string, serviceType enums.ServiceType) (*models.InstalledService, error)
	FindServiceByServerAndSoftware(ctx context.Context, serverID string, software enums.Software) (*models.InstalledService, error)
	FindDatabaseService(ctx context.Context, serverID string) (*models.InstalledService, error)
	UpdateService(ctx context.Context, service *models.InstalledService) error
	UpdateServiceStatus(ctx context.Context, id string, status enums.ServiceStatus) error
	DeleteService(ctx context.Context, id string) error
}

// FirewallRuleRepository defines the interface for firewall rule database operations
type FirewallRuleRepository interface {
	CreateFirewallRule(ctx context.Context, rule *models.FirewallRule) error
	FindFirewallRuleByID(ctx context.Context, id string) (*models.FirewallRule, error)
	FindFirewallRuleByIDAndServer(ctx context.Context, id, serverID string) (*models.FirewallRule, error)
	FindFirewallRulesByServer(ctx context.Context, serverID string) ([]models.FirewallRule, error)
	UpdateFirewallRule(ctx context.Context, rule *models.FirewallRule) error
	MarkFirewallRuleInstalled(ctx context.Context, id string) error
	MarkFirewallRuleFailed(ctx context.Context, id string) error
	DeleteFirewallRule(ctx context.Context, id string) error
}

// CronRepository defines the interface for cron job database operations
type CronRepository interface {
	CreateCron(ctx context.Context, cron *models.Cron) error
	FindCronByID(ctx context.Context, id string) (*models.Cron, error)
	FindCronByIDAndServer(ctx context.Context, id, serverID string) (*models.Cron, error)
	FindCronsByServer(ctx context.Context, serverID string) ([]models.Cron, error)
	FindVisibleCronsByServer(ctx context.Context, serverID string) ([]models.Cron, error)
	UpdateCron(ctx context.Context, cron *models.Cron) error
	MarkCronInstalled(ctx context.Context, id string) error
	DeleteCron(ctx context.Context, id string) error
}

// DaemonRepository defines the interface for daemon database operations
type DaemonRepository interface {
	CreateDaemon(ctx context.Context, daemon *models.Daemon) error
	FindDaemonByID(ctx context.Context, id string) (*models.Daemon, error)
	FindDaemonByIDAndServer(ctx context.Context, id, serverID string) (*models.Daemon, error)
	FindDaemonsByServer(ctx context.Context, serverID string) ([]models.Daemon, error)
	UpdateDaemon(ctx context.Context, daemon *models.Daemon) error
	UpdateDaemonStatus(ctx context.Context, id string, running bool) error
	MarkDaemonInstalled(ctx context.Context, id string) error
	DeleteDaemon(ctx context.Context, id string) error
}

// SshKeyRepository defines the interface for SSH key database operations
type SshKeyRepository interface {
	CreateSshKey(ctx context.Context, key *models.SshKey) error
	FindSshKeyByID(ctx context.Context, id string) (*models.SshKey, error)
	FindSshKeysByTeam(ctx context.Context, teamID string) ([]models.SshKey, error)
	FindSshKeysByServer(ctx context.Context, serverID string) ([]models.SshKey, error)
	FindGlobalSshKeys(ctx context.Context) ([]models.SshKey, error)
	UpdateSshKey(ctx context.Context, key *models.SshKey) error
	DeleteSshKey(ctx context.Context, id string) error
	AttachSshKeyToServer(ctx context.Context, serverID, sshKeyID string) error
	DetachSshKeyFromServer(ctx context.Context, serverID, sshKeyID string) error
	IsSshKeyAttachedToServer(ctx context.Context, serverID, sshKeyID string) (bool, error)
}

// TaskRepository defines the interface for task database operations
type TaskRepository interface {
	CreateTask(ctx context.Context, task *models.Task) error
	FindTaskByID(ctx context.Context, id string) (*models.Task, error)
	FindTasksByServer(ctx context.Context, serverID string, limit int) ([]models.Task, error)
	FindLatestTaskByServer(ctx context.Context, serverID string) (*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error
}

// MetricRepository defines the interface for metric database operations
type MetricRepository interface {
	CreateMetric(ctx context.Context, metric *models.Metric) error
	FindMetricsByServer(ctx context.Context, serverID string, from, to *time.Time, limit int) ([]models.Metric, error)
	FindLatestMetricByServer(ctx context.Context, serverID string) (*models.Metric, error)
	DeleteOldMetrics(ctx context.Context, serverID string, before time.Time) error
}

// Repository combines all repository interfaces
type Repository interface {
	ServerRepository
	ServiceRepository
	FirewallRuleRepository
	CronRepository
	DaemonRepository
	SshKeyRepository
	TaskRepository
	MetricRepository
	Transaction(ctx context.Context, fn func(tx Repository) error) error
}
