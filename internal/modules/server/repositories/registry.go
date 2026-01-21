package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"gorm.io/gorm"
)

// Registry holds all server module repositories
type Registry struct {
	db             *gorm.DB
	server         *ServerRepository
	service        *ServiceRepository
	firewallRule   *FirewallRuleRepository
	cron           *CronRepository
	daemon         *DaemonRepository
	sshKey         *SSHKeyRepository
	task           *TaskRepository
	metric         *MetricRepository
	serverProvider *ServerProviderRepository
	database       *DatabaseRepository
}

// NewRegistry creates all repositories
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		db:             db,
		server:         NewServerRepository(db),
		service:        NewServiceRepository(db),
		firewallRule:   NewFirewallRuleRepository(db),
		cron:           NewCronRepository(db),
		daemon:         NewDaemonRepository(db),
		sshKey:         NewSSHKeyRepository(db),
		task:           NewTaskRepository(db),
		metric:         NewMetricRepository(db),
		serverProvider: NewServerProviderRepository(db),
		database:       NewDatabaseRepository(db),
	}
}

// Server returns the server repository
func (r *Registry) Server() contracts.ServerRepository { return r.server }

// Service returns the service repository
func (r *Registry) Service() contracts.ServiceRepository { return r.service }

// FirewallRule returns the firewall rule repository
func (r *Registry) FirewallRule() contracts.FirewallRuleRepository { return r.firewallRule }

// Cron returns the cron repository
func (r *Registry) Cron() contracts.CronRepository { return r.cron }

// Daemon returns the daemon repository
func (r *Registry) Daemon() contracts.DaemonRepository { return r.daemon }

// SshKey returns the SSH key repository
func (r *Registry) SSHKey() contracts.SSHKeyRepository { return r.sshKey }

// Task returns the task repository
func (r *Registry) Task() contracts.TaskRepository { return r.task }

// Metric returns the metric repository
func (r *Registry) Metric() contracts.MetricRepository { return r.metric }

// ServerProvider returns the server provider repository
func (r *Registry) ServerProvider() contracts.ServerProviderRepository { return r.serverProvider }

// Database returns the database repository
func (r *Registry) Database() contracts.DatabaseRepository { return r.database }

// DB returns the underlying database connection for activity logging
func (r *Registry) DB() *gorm.DB { return r.db }

// FindTaskByID implements the TaskWebhookRepository interface
func (r *Registry) FindTaskByID(ctx context.Context, id string) (*models.Task, error) {
	return r.task.FindByID(ctx, id)
}

// UpdateTask implements the TaskWebhookRepository interface
func (r *Registry) UpdateTask(ctx context.Context, task *models.Task) error {
	return r.task.Update(ctx, task)
}

// FindServerByID implements the ProvisionScriptRepository interface
func (r *Registry) FindServerByID(ctx context.Context, id string) (*models.Server, error) {
	return r.server.FindByID(ctx, id)
}
