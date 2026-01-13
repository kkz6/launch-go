// Package jobs provides async job definitions for server operations
package jobs

// Job type constants - these identify each job type in the queue
const (
	TypeCreateOnProvider    = "server:create_on_provider"
	TypeProvisionServer     = "server:provision"
	TypeDeleteServer        = "server:delete"
	TypeRebootServer        = "server:reboot"
	TypeInstallCron         = "server:install_cron"
	TypeUninstallCron       = "server:uninstall_cron"
	TypeInstallDaemon       = "server:install_daemon"
	TypeUninstallDaemon     = "server:uninstall_daemon"
	TypeRestartDaemon       = "server:restart_daemon"
	TypeInstallFirewallRule = "server:install_firewall_rule"
	TypeUninstallFirewall   = "server:uninstall_firewall_rule"
	TypeAddService          = "server:add_service"
	TypeRemoveService       = "server:remove_service"
	TypeServiceOperation    = "server:service_operation"
	TypeAddSshKey           = "server:add_ssh_key"
	TypeRemoveSshKey        = "server:remove_ssh_key"
	TypeUpdateConnectivity  = "server:update_connectivity"
	TypeArchiveServer       = "server:archive"
	TypeUnarchiveServer     = "server:unarchive"
)

// CreateOnProviderPayload is the payload for creating a server on the cloud provider
type CreateOnProviderPayload struct {
	ServerID         string   `json:"server_id"`
	TeamID           string   `json:"team_id"`
	UserID           *string  `json:"user_id,omitempty"`
	SSHKeyIDs        []string `json:"ssh_key_ids,omitempty"`
	ServerProviderID string   `json:"server_provider_id,omitempty"`
}

// ProvisionServerPayload is the payload for the provision server job
type ProvisionServerPayload struct {
	ServerID  string   `json:"server_id"`
	TeamID    string   `json:"team_id"`
	UserID    *string  `json:"user_id,omitempty"`
	SSHKeyIDs []string `json:"ssh_key_ids,omitempty"`
}

// DeleteServerPayload is the payload for the delete server job
type DeleteServerPayload struct {
	ServerID string  `json:"server_id"`
	TeamID   string  `json:"team_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// RebootServerPayload is the payload for the reboot server job
type RebootServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallCronPayload is the payload for the install cron job
type InstallCronPayload struct {
	ServerID string  `json:"server_id"`
	CronID   string  `json:"cron_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UninstallCronPayload is the payload for the uninstall cron job
type UninstallCronPayload struct {
	ServerID string  `json:"server_id"`
	CronID   string  `json:"cron_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallDaemonPayload is the payload for the install daemon job
type InstallDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UninstallDaemonPayload is the payload for the uninstall daemon job
type UninstallDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// RestartDaemonPayload is the payload for the restart daemon job
type RestartDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallFirewallRulePayload is the payload for the install firewall rule job
type InstallFirewallRulePayload struct {
	ServerID string  `json:"server_id"`
	RuleID   string  `json:"rule_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UninstallFirewallRulePayload is the payload for the uninstall firewall rule job
type UninstallFirewallRulePayload struct {
	ServerID string  `json:"server_id"`
	RuleID   string  `json:"rule_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// AddServicePayload is the payload for the add service job
type AddServicePayload struct {
	ServerID  string `json:"server_id"`
	ServiceID string `json:"service_id"`
	Software  string `json:"software"`
}

// RemoveServicePayload is the payload for the remove service job
type RemoveServicePayload struct {
	ServerID  string  `json:"server_id"`
	ServiceID string  `json:"service_id"`
	UserID    *string `json:"user_id,omitempty"`
}

// ServiceOperationPayload is the payload for service operations (start/stop/restart)
type ServiceOperationPayload struct {
	ServerID  string  `json:"server_id"`
	ServiceID string  `json:"service_id"`
	Operation string  `json:"operation"` // start, stop, restart, reload
	UserID    *string `json:"user_id,omitempty"`
}

// AddSshKeyPayload is the payload for the add SSH key job
type AddSshKeyPayload struct {
	ServerID string `json:"server_id"`
	KeyID    string `json:"key_id"`
}

// RemoveSshKeyPayload is the payload for the remove SSH key job
type RemoveSshKeyPayload struct {
	ServerID string `json:"server_id"`
	KeyID    string `json:"key_id"`
	Force    bool   `json:"force"`
}

// UpdateConnectivityPayload is the payload for the update connectivity job
type UpdateConnectivityPayload struct {
	ServerID string `json:"server_id"`
}

// ArchiveServerPayload is the payload for the archive server job
type ArchiveServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UnarchiveServerPayload is the payload for the unarchive server job
type UnarchiveServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}
