// Package jobs provides async job definitions for server operations
package jobs

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// JobContext holds dependencies for server jobs
type JobContext struct {
	DB         *gorm.DB
	Repo       contracts.Repository
	Logger     *zerolog.Logger
	WS         jobs.Broadcaster
	Dispatcher *taskrunner.Dispatcher
	TaskRepo   contracts.Repository
}

// NewJobContext creates a new job context
func NewJobContext(
	db *gorm.DB,
	repo contracts.Repository,
	logger *zerolog.Logger,
	ws jobs.Broadcaster,
	dispatcher *taskrunner.Dispatcher,
	taskRepo contracts.Repository,
) *JobContext {
	return &JobContext{
		DB:         db,
		Repo:       repo,
		Logger:     logger,
		WS:         ws,
		Dispatcher: dispatcher,
		TaskRepo:   taskRepo,
	}
}

// Registry handles job registration
type Registry struct {
	ctx *JobContext
}

// NewRegistry creates a new job registry
func NewRegistry(ctx *JobContext) *Registry {
	return &Registry{ctx: ctx}
}

// RegisterHandlers registers all job handlers
func (r *Registry) RegisterHandlers(mux *asynq.ServeMux) {
	// TODO: Register job handlers
}

// Task type constants
const (
	TypeProvisionServer     = "server:provision"
	TypeDeleteServer        = "server:delete"
	TypeRebootServer        = "server:reboot"
	TypeInstallCron         = "server:install_cron"
	TypeUninstallCron       = "server:uninstall_cron"
	TypeInstallDaemon       = "server:install_daemon"
	TypeUninstallDaemon     = "server:uninstall_daemon"
	TypeInstallFirewallRule = "server:install_firewall_rule"
	TypeUninstallFirewall   = "server:uninstall_firewall_rule"
	TypeAddService          = "server:add_service"
	TypeServiceOperation    = "server:service_operation"
	TypeAddSshKey           = "server:add_ssh_key"
	TypeRemoveSshKey        = "server:remove_ssh_key"
)

// NewProvisionServerTask creates a provision server job
func NewProvisionServerTask(serverID, teamID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeProvisionServer, map[string]interface{}{
		"server_id": serverID,
		"team_id":   teamID,
		"user_id":   userID,
	})
}

// NewDeleteServerTask creates a delete server job
func NewDeleteServerTask(serverID, teamID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeDeleteServer, map[string]interface{}{
		"server_id": serverID,
		"team_id":   teamID,
		"user_id":   userID,
	})
}

// NewRebootServerTask creates a reboot server job
func NewRebootServerTask(serverID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeRebootServer, map[string]interface{}{
		"server_id": serverID,
		"user_id":   userID,
	})
}

// NewInstallCronTask creates an install cron job
func NewInstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallCron, map[string]interface{}{
		"server_id": serverID,
		"cron_id":   cronID,
		"user_id":   userID,
	})
}

// NewUninstallCronTask creates an uninstall cron job
func NewUninstallCronTask(serverID, cronID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallCron, map[string]interface{}{
		"server_id": serverID,
		"cron_id":   cronID,
		"user_id":   userID,
	})
}

// NewInstallDaemonTask creates an install daemon job
func NewInstallDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallDaemon, map[string]interface{}{
		"server_id": serverID,
		"daemon_id": daemonID,
		"user_id":   userID,
	})
}

// NewUninstallDaemonTask creates an uninstall daemon job
func NewUninstallDaemonTask(serverID, daemonID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallDaemon, map[string]interface{}{
		"server_id": serverID,
		"daemon_id": daemonID,
		"user_id":   userID,
	})
}

// NewInstallFirewallRuleTask creates an install firewall rule job
func NewInstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallFirewallRule, map[string]interface{}{
		"server_id": serverID,
		"rule_id":   ruleID,
		"user_id":   userID,
	})
}

// NewUninstallFirewallRuleTask creates an uninstall firewall rule job
func NewUninstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallFirewall, map[string]interface{}{
		"server_id": serverID,
		"rule_id":   ruleID,
		"user_id":   userID,
	})
}

// NewAddServiceTask creates an add service job
func NewAddServiceTask(serverID, serviceID string, software enums.Software) (*asynq.Task, error) {
	return jobs.NewTask(TypeAddService, map[string]interface{}{
		"server_id":  serverID,
		"service_id": serviceID,
		"software":   software,
	})
}

// NewServiceOperationTask creates a service operation job
func NewServiceOperationTask(serverID, serviceID, operation string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeServiceOperation, map[string]interface{}{
		"server_id":  serverID,
		"service_id": serviceID,
		"operation":  operation,
		"user_id":    userID,
	})
}

// NewAddSshKeyTask creates an add SSH key job
func NewAddSshKeyTask(serverID, keyID string) (*asynq.Task, error) {
	return jobs.NewTask(TypeAddSshKey, map[string]interface{}{
		"server_id": serverID,
		"key_id":    keyID,
	})
}

// NewRemoveSshKeyTask creates a remove SSH key job
func NewRemoveSshKeyTask(serverID, keyID string, force bool) (*asynq.Task, error) {
	return jobs.NewTask(TypeRemoveSshKey, map[string]interface{}{
		"server_id": serverID,
		"key_id":    keyID,
		"force":     force,
	})
}
