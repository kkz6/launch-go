package contracts

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ServerService defines the interface for server business logic
type ServerService interface {
	ListServers(ctx context.Context, teamID string) ([]models.Server, error)
	ListServersPaginated(ctx context.Context, teamID string, page, perPage int) (*repository.PaginatedResult[models.Server], error)
	GetServer(ctx context.Context, id, teamID string) (*models.Server, error)
	GetServerWithRelations(ctx context.Context, id, teamID string) (*models.Server, error)
	CreateServer(ctx context.Context, teamID, userID string, req *dto.CreateServerRequest) (*models.Server, error)
	UpdateServer(ctx context.Context, id, teamID string, req *dto.UpdateServerRequest) (*models.Server, error)
	DeleteServer(ctx context.Context, id, teamID string) error
	ArchiveServer(ctx context.Context, id, teamID string) error
	UnarchiveServer(ctx context.Context, id, teamID string) error
	RebootServer(ctx context.Context, id, teamID string) error
	ConnectServer(ctx context.Context, id, teamID string) error
	HasLaunchAgent(ctx context.Context, serverID string) (bool, error)
	GetShowPageData(ctx context.Context, serverID, teamID string) (*dto.ServerShowPageData, error)
}

// InstalledServiceService defines the interface for installed service business logic
type InstalledServiceService interface {
	ListServices(ctx context.Context, serverID, teamID string) ([]models.InstalledService, error)
	InstallService(ctx context.Context, serverID, teamID string, req *dto.CreateServiceRequest) (*models.InstalledService, error)
	HandleServiceOperation(ctx context.Context, serverID, teamID, serviceID string, operation types.ServiceOption) error
}

// FirewallRuleService defines the interface for firewall rule business logic
type FirewallRuleService interface {
	ListFirewallRules(ctx context.Context, serverID, teamID string) ([]models.FirewallRule, error)
	CreateFirewallRule(ctx context.Context, serverID, teamID string, req *dto.CreateFirewallRuleRequest) (*models.FirewallRule, error)
	UpdateFirewallRule(ctx context.Context, serverID, teamID, ruleID string, req *dto.UpdateFirewallRuleRequest) (*models.FirewallRule, error)
	DeleteFirewallRule(ctx context.Context, serverID, teamID, ruleID string) error
}

// CronService defines the interface for cron job business logic
type CronService interface {
	ListCrons(ctx context.Context, serverID, teamID string) ([]models.Cron, error)
	CreateCron(ctx context.Context, serverID, teamID string, req *dto.CreateCronRequest) (*models.Cron, error)
	UpdateCron(ctx context.Context, serverID, teamID, cronID string, req *dto.UpdateCronRequest) (*models.Cron, error)
	DeleteCron(ctx context.Context, serverID, teamID, cronID string) error
}

// DaemonService defines the interface for daemon business logic
type DaemonService interface {
	ListDaemons(ctx context.Context, serverID, teamID string) ([]models.Daemon, error)
	CreateDaemon(ctx context.Context, serverID, teamID string, req *dto.CreateDaemonRequest) (*models.Daemon, error)
	UpdateDaemon(ctx context.Context, serverID, teamID, daemonID string, req *dto.UpdateDaemonRequest) (*models.Daemon, error)
	DeleteDaemon(ctx context.Context, serverID, teamID, daemonID string) error
}

// SSHKeyService defines the interface for SSH key business logic
type SSHKeyService interface {
	ListSSHKeys(ctx context.Context, teamID string) ([]models.SSHKey, error)
	ListServerSSHKeys(ctx context.Context, serverID, teamID string) ([]models.SSHKey, error)
	CreateSSHKey(ctx context.Context, teamID, userID string, req *dto.CreateSSHKeyRequest) (*models.SSHKey, error)
	AttachSSHKey(ctx context.Context, serverID, teamID, sshKeyID string) error
	DetachSSHKey(ctx context.Context, serverID, teamID, sshKeyID string) error
	DeleteSSHKey(ctx context.Context, teamID, sshKeyID string) error
}

// TaskService defines the interface for task business logic
type TaskService interface {
	ListTasks(ctx context.Context, serverID, teamID string, limit int) ([]models.Task, error)
	GetLatestTask(ctx context.Context, serverID, teamID string) (*models.Task, error)
}

// MetricService defines the interface for metric business logic
type MetricService interface {
	GetLatestMetric(ctx context.Context, serverID, teamID string) (*models.Metric, error)
	GetMetrics(ctx context.Context, serverID, teamID string, from, to *time.Time, limit int) ([]models.Metric, error)
}

// DatabaseService defines the interface for database business logic
type DatabaseService interface {
	ListDatabases(ctx context.Context, serverID, teamID string) ([]dto.DatabaseResponse, error)
	CreateDatabase(ctx context.Context, serverID, teamID string, req *dto.CreateDatabaseRequest) (*dto.DatabaseResponse, error)
}

// Service combines all service interfaces
type Service interface {
	ServerService
	InstalledServiceService
	FirewallRuleService
	CronService
	DaemonService
	SSHKeyService
	TaskService
	MetricService
	DatabaseService
}
