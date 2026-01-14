package jobs

const (
	TypeCreateOnProvider     = "server:create_on_provider"
	TypeProvisionServer      = "server:provision"
	TypeDeleteServer         = "server:delete"
	TypeRebootServer         = "server:reboot"
	TypeInstallCron          = "server:install_cron"
	TypeUninstallCron        = "server:uninstall_cron"
	TypeInstallDaemon        = "server:install_daemon"
	TypeUninstallDaemon      = "server:uninstall_daemon"
	TypeRestartDaemon        = "server:restart_daemon"
	TypeInstallFirewallRule  = "server:install_firewall_rule"
	TypeUninstallFirewall    = "server:uninstall_firewall_rule"
	TypeAddService           = "server:add_service"
	TypeRemoveService        = "server:remove_service"
	TypeServiceOperation     = "server:service_operation"
	TypeAddSshKey            = "server:add_ssh_key"
	TypeRemoveSshKey         = "server:remove_ssh_key"
	TypeUpdateConnectivity   = "server:update_connectivity"
	TypeArchiveServer        = "server:archive"
	TypeUnarchiveServer      = "server:unarchive"
	TypeConfigureOpcache     = "server:configure_opcache"
	TypeVulnerabilityAudit   = "server:vulnerability_audit"
)

type CreateOnProviderPayload struct {
	ServerID         string   `json:"server_id"`
	TeamID           string   `json:"team_id"`
	UserID           *string  `json:"user_id,omitempty"`
	SSHKeyIDs        []string `json:"ssh_key_ids,omitempty"`
	ServerProviderID string   `json:"server_provider_id,omitempty"`
}

type ProvisionServerPayload struct {
	ServerID  string   `json:"server_id"`
	TeamID    string   `json:"team_id"`
	UserID    *string  `json:"user_id,omitempty"`
	SSHKeyIDs []string `json:"ssh_key_ids,omitempty"`
}

type DeleteServerPayload struct {
	ServerID string  `json:"server_id"`
	TeamID   string  `json:"team_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type RebootServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type InstallCronPayload struct {
	ServerID string  `json:"server_id"`
	CronID   string  `json:"cron_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type UninstallCronPayload struct {
	ServerID string  `json:"server_id"`
	CronID   string  `json:"cron_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type InstallDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type UninstallDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type RestartDaemonPayload struct {
	ServerID string  `json:"server_id"`
	DaemonID string  `json:"daemon_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type InstallFirewallRulePayload struct {
	ServerID string  `json:"server_id"`
	RuleID   string  `json:"rule_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type UninstallFirewallRulePayload struct {
	ServerID string  `json:"server_id"`
	RuleID   string  `json:"rule_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type AddServicePayload struct {
	ServerID  string `json:"server_id"`
	ServiceID string `json:"service_id"`
	Software  string `json:"software"`
}

type RemoveServicePayload struct {
	ServerID  string  `json:"server_id"`
	ServiceID string  `json:"service_id"`
	UserID    *string `json:"user_id,omitempty"`
}

type ServiceOperationPayload struct {
	ServerID  string  `json:"server_id"`
	ServiceID string  `json:"service_id"`
	Operation string  `json:"operation"`
	UserID    *string `json:"user_id,omitempty"`
}

type AddSshKeyPayload struct {
	ServerID string `json:"server_id"`
	KeyID    string `json:"key_id"`
}

type RemoveSshKeyPayload struct {
	ServerID string `json:"server_id"`
	KeyID    string `json:"key_id"`
	Force    bool   `json:"force"`
}

type UpdateConnectivityPayload struct {
	ServerID string `json:"server_id"`
}

type ArchiveServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type UnarchiveServerPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

type ConfigureOpcachePayload struct {
	ServerID  string            `json:"server_id"`
	ServiceID string            `json:"service_id"`
	Settings  map[string]string `json:"settings"`
}

type VulnerabilityAuditPayload struct {
	ServerID       string  `json:"server_id"`
	TeamID         string  `json:"team_id"`
	UserID         *string `json:"user_id,omitempty"`
	EmailRecipient *string `json:"email_recipient,omitempty"`
}
