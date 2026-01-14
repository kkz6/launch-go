package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// Task type constants
const (
	TypeDeploy              = "site:deploy"
	TypeDeployZeroDowntime  = "site:deploy_zero_downtime"
	TypeRollback            = "site:rollback"
	TypeInstallSSL          = "site:install_ssl"
	TypeRunCommand          = "site:run_command"
	TypeInstallQueue        = "site:install_queue"
	TypeRestartQueue        = "site:restart_queue"
	TypeUninstallQueue      = "site:uninstall_queue"
	TypeSyncQueues          = "site:sync_queues"
	TypeInstallRedirect     = "site:install_redirect"
	TypeUninstallRedirect   = "site:uninstall_redirect"
	TypeInstallCaddyfile    = "site:install_caddyfile"
	TypeUninstallCaddyfile  = "site:uninstall_caddyfile"
	TypeUpdateCaddyfile     = "site:update_caddyfile"
)

// RunCommandPayload holds data for running a command on a site
type RunCommandPayload struct {
	SiteID    string `json:"site_id"`
	CommandID string `json:"command_id"`
}

// DeployPayload holds data for site deployment
type DeployPayload struct {
	SiteID       string  `json:"site_id"`
	DeploymentID string  `json:"deployment_id"`
	UserID       *string `json:"user_id,omitempty"`
}

// RollbackPayload holds data for deployment rollback
type RollbackPayload struct {
	SiteID             string  `json:"site_id"`
	DeploymentID       string  `json:"deployment_id"`
	TargetDeploymentID string  `json:"target_deployment_id"`
	UserID             *string `json:"user_id,omitempty"`
}

// InstallSSLPayload holds data for SSL installation
type InstallSSLPayload struct {
	SiteID  string `json:"site_id"`
	Address string `json:"address"`
}

// InstallQueuePayload holds data for queue worker installation
type InstallQueuePayload struct {
	SiteID  string  `json:"site_id"`
	QueueID string  `json:"queue_id"`
	UserID  *string `json:"user_id,omitempty"`
}

// RestartQueuePayload holds data for queue worker restart
type RestartQueuePayload struct {
	SiteID  string  `json:"site_id"`
	QueueID string  `json:"queue_id"`
	UserID  *string `json:"user_id,omitempty"`
}

// UninstallQueuePayload holds data for queue worker uninstallation
type UninstallQueuePayload struct {
	SiteID  string  `json:"site_id"`
	QueueID string  `json:"queue_id"`
	UserID  *string `json:"user_id,omitempty"`
}

// SyncQueuesPayload holds data for queue status synchronization
type SyncQueuesPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallRedirectPayload holds data for redirect installation
type InstallRedirectPayload struct {
	SiteID     string  `json:"site_id"`
	RedirectID string  `json:"redirect_id"`
	UserID     *string `json:"user_id,omitempty"`
}

// UninstallRedirectPayload holds data for redirect uninstallation
type UninstallRedirectPayload struct {
	SiteID     string  `json:"site_id"`
	RedirectID string  `json:"redirect_id"`
	UserID     *string `json:"user_id,omitempty"`
}

// CaddyfilePayload holds data for Caddyfile operations
type CaddyfilePayload struct {
	SiteID string  `json:"site_id"`
	UserID *string `json:"user_id,omitempty"`
}

// NewDeployTask creates a deploy job
func NewDeployTask(siteID, deploymentID string, userID string) (*asynq.Task, error) {
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}
	return jobs.NewTask(TypeDeploy, DeployPayload{
		SiteID:       siteID,
		DeploymentID: deploymentID,
		UserID:       userIDPtr,
	})
}

// NewDeployZeroDowntimeTask creates a zero-downtime deploy job
func NewDeployZeroDowntimeTask(siteID, deploymentID string, userID string) (*asynq.Task, error) {
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}
	return jobs.NewTask(TypeDeployZeroDowntime, DeployPayload{
		SiteID:       siteID,
		DeploymentID: deploymentID,
		UserID:       userIDPtr,
	})
}

// NewRollbackTask creates a rollback job
func NewRollbackTask(siteID, deploymentID, targetDeploymentID, userID string) (*asynq.Task, error) {
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}
	return jobs.NewTask(TypeRollback, RollbackPayload{
		SiteID:             siteID,
		DeploymentID:       deploymentID,
		TargetDeploymentID: targetDeploymentID,
		UserID:             userIDPtr,
	})
}

// NewInstallSSLTask creates an install SSL job
func NewInstallSSLTask(siteID, address string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallSSL, InstallSSLPayload{
		SiteID:  siteID,
		Address: address,
	})
}

// NewRunCommandTask creates a run command job
func NewRunCommandTask(siteID, commandID string) (*asynq.Task, error) {
	return jobs.NewTask(TypeRunCommand, RunCommandPayload{
		SiteID:    siteID,
		CommandID: commandID,
	})
}

// NewInstallQueueTask creates an install queue job
func NewInstallQueueTask(siteID, queueID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallQueue, InstallQueuePayload{
		SiteID:  siteID,
		QueueID: queueID,
		UserID:  userID,
	})
}

// NewRestartQueueTask creates a restart queue job
func NewRestartQueueTask(siteID, queueID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeRestartQueue, RestartQueuePayload{
		SiteID:  siteID,
		QueueID: queueID,
		UserID:  userID,
	})
}

// NewUninstallQueueTask creates an uninstall queue job
func NewUninstallQueueTask(siteID, queueID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallQueue, UninstallQueuePayload{
		SiteID:  siteID,
		QueueID: queueID,
		UserID:  userID,
	})
}

// NewSyncQueuesTask creates a sync queues status job
func NewSyncQueuesTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeSyncQueues, SyncQueuesPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}

// NewInstallRedirectTask creates an install redirect job
func NewInstallRedirectTask(siteID, redirectID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallRedirect, InstallRedirectPayload{
		SiteID:     siteID,
		RedirectID: redirectID,
		UserID:     userID,
	})
}

// NewUninstallRedirectTask creates an uninstall redirect job
func NewUninstallRedirectTask(siteID, redirectID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallRedirect, UninstallRedirectPayload{
		SiteID:     siteID,
		RedirectID: redirectID,
		UserID:     userID,
	})
}

// NewInstallCaddyfileTask creates an install Caddyfile job
func NewInstallCaddyfileTask(siteID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallCaddyfile, CaddyfilePayload{
		SiteID: siteID,
		UserID: userID,
	})
}

// NewUninstallCaddyfileTask creates an uninstall Caddyfile job
func NewUninstallCaddyfileTask(siteID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallCaddyfile, CaddyfilePayload{
		SiteID: siteID,
		UserID: userID,
	})
}

// NewUpdateCaddyfileTask creates an update Caddyfile job
func NewUpdateCaddyfileTask(siteID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUpdateCaddyfile, CaddyfilePayload{
		SiteID: siteID,
		UserID: userID,
	})
}
