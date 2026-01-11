package jobs

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/websocket"
)

// Legacy task types - kept for backward compatibility
// Use the individual job type constants instead (e.g., TypeProvisionServer)
const (
	TypeProvision         = "server:provision"          // Deprecated: Use TypeProvisionServer
	TypeInstallPHP        = "server:install_php"        // Deprecated: Use TypeAddPhpVersion
	TypeConfigureFirewall = "server:configure_firewall" // Use firewall rule jobs instead
	TypeReboot = "server:reboot"
	TypeDelete            = "server:delete"
	TypeInstallService    = "server:install_service" // Deprecated: Use TypeAddService
	TypeServiceOperation  = "server:service_operation"
	TypeFirewallRule      = "server:firewall_rule" // Deprecated: Use TypeInstallFirewallRule
	TypeCron              = "server:cron"          // Deprecated: Use TypeInstallCron
	TypeDaemon            = "server:daemon"        // Deprecated: Use TypeInstallDaemon
	TypeSshKey            = "server:ssh_key"       // Deprecated: Use TypeAddSshKey/TypeRemoveSshKey
)

// Handler provides legacy job handling (deprecated - use individual job handlers)
type Handler struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewHandler creates a new handler (deprecated - use NewRegistry instead)
func NewHandler(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *Handler {
	return &Handler{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// RebootPayload contains data for rebooting a server
type RebootPayload struct {
	ServerID string `json:"server_id"`
}

// NewRebootTask creates a new asynq task for rebooting a server
func NewRebootTask(serverID string) (*asynq.Task, error) {
	payload, err := json.Marshal(RebootPayload{ServerID: serverID})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeReboot, payload), nil
}

// HandleReboot handles a server reboot task
func (h *Handler) HandleReboot(ctx context.Context, t *asynq.Task) error {
	var payload RebootPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("server_id", payload.ServerID).Msg("Rebooting server")

	// TODO: Implement server reboot

	return nil
}

// DeletePayload contains data for deleting a server
type DeletePayload struct {
	ServerID string `json:"server_id"`
	TeamID   string `json:"team_id"`
}

// NewDeleteTask creates a new asynq task for deleting a server
func NewDeleteTask(serverID, teamID string) (*asynq.Task, error) {
	payload, err := json.Marshal(DeletePayload{
		ServerID: serverID,
		TeamID:   teamID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeDelete, payload), nil
}

// HandleDelete handles a server deletion task
func (h *Handler) HandleDelete(ctx context.Context, t *asynq.Task) error {
	var payload DeletePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("server_id", payload.ServerID).Msg("Deleting server")

	// TODO: Implement server deletion on cloud provider

	return nil
}

// ConfigureFirewallPayload contains data for configuring firewall
type ConfigureFirewallPayload struct {
	ServerID string `json:"server_id"`
}

// NewConfigureFirewallTask creates a new asynq task for configuring firewall
func NewConfigureFirewallTask(serverID string) (*asynq.Task, error) {
	payload, err := json.Marshal(ConfigureFirewallPayload{ServerID: serverID})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeConfigureFirewall, payload), nil
}

// HandleConfigureFirewall handles a firewall configuration task
func (h *Handler) HandleConfigureFirewall(ctx context.Context, t *asynq.Task) error {
	var payload ConfigureFirewallPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("server_id", payload.ServerID).Msg("Configuring firewall")

	// TODO: Implement firewall configuration

	return nil
}

// HandleProvision handles a server provisioning task
func (h *Handler) HandleProvision(ctx context.Context, t *asynq.Task) error {
	var payload ProvisionTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("server_id", payload.ServerID).Msg("Provisioning server")

	// TODO: Implement server provisioning

	return nil
}

// InstallPHPPayload contains data for PHP installation
type InstallPHPPayload struct {
	ServerID   string `json:"server_id"`
	PHPVersion string `json:"php_version"`
}

// HandleInstallPHP handles a PHP installation task
func (h *Handler) HandleInstallPHP(ctx context.Context, t *asynq.Task) error {
	var payload InstallPHPPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().
		Str("server_id", payload.ServerID).
		Str("php_version", payload.PHPVersion).
		Msg("Installing PHP")

	// TODO: Implement PHP installation

	return nil
}

// ServiceOperationPayload contains data for service operations
type ServiceOperationPayload struct {
	ServerID  string `json:"server_id"`
	ServiceID string `json:"service_id"`
	Operation string `json:"operation"`
}

// NewServiceOperationTask creates a new asynq task for service operations
func NewServiceOperationTask(serverID, serviceID, operation string) (*asynq.Task, error) {
	payload, err := json.Marshal(ServiceOperationPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Operation: operation,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeServiceOperation, payload), nil
}

// HandleServiceOperation handles a service operation task
func (h *Handler) HandleServiceOperation(ctx context.Context, t *asynq.Task) error {
	var payload ServiceOperationPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().
		Str("server_id", payload.ServerID).
		Str("service_id", payload.ServiceID).
		Str("operation", payload.Operation).
		Msg("Service operation")

	// TODO: Implement service operations

	return nil
}

// broadcastProgress sends a progress update via WebSocket
func (h *Handler) broadcastProgress(serverID, status, message string) {
	h.ws.BroadcastToServer(serverID, "server.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}

// Convenience functions for services

// CronTaskPayload contains data for cron tasks
type CronTaskPayload struct {
	ServerID string `json:"server_id"`
	CronID   string `json:"cron_id"`
	Action   string `json:"action"`
}

// NewCronTask creates a new cron task (install or uninstall)
func NewCronTask(serverID, cronID, action string) (*asynq.Task, error) {
	payload, err := json.Marshal(CronTaskPayload{
		ServerID: serverID,
		CronID:   cronID,
		Action:   action,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeCron, payload), nil
}

// DaemonTaskPayload contains data for daemon tasks
type DaemonTaskPayload struct {
	ServerID string `json:"server_id"`
	DaemonID string `json:"daemon_id"`
	Action   string `json:"action"`
}

// NewDaemonTask creates a new daemon task (install or uninstall)
func NewDaemonTask(serverID, daemonID, action string) (*asynq.Task, error) {
	payload, err := json.Marshal(DaemonTaskPayload{
		ServerID: serverID,
		DaemonID: daemonID,
		Action:   action,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeDaemon, payload), nil
}

// FirewallRuleTaskPayload contains data for firewall rule tasks
type FirewallRuleTaskPayload struct {
	ServerID string `json:"server_id"`
	RuleID   string `json:"rule_id"`
	Action   string `json:"action"`
}

// NewFirewallRuleTask creates a new firewall rule task (install or uninstall)
func NewFirewallRuleTask(serverID, ruleID, action string) (*asynq.Task, error) {
	payload, err := json.Marshal(FirewallRuleTaskPayload{
		ServerID: serverID,
		RuleID:   ruleID,
		Action:   action,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeFirewallRule, payload), nil
}

// InstallServiceTaskPayload contains data for service installation
type InstallServiceTaskPayload struct {
	ServerID  string `json:"server_id"`
	ServiceID string `json:"service_id"`
	Software  string `json:"software"`
}

// NewInstallServiceTask creates a new service installation task
func NewInstallServiceTask(serverID, serviceID, software string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallServiceTaskPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Software:  software,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeInstallService, payload), nil
}

// ProvisionTaskPayload contains data for server provisioning
type ProvisionTaskPayload struct {
	ServerID string `json:"server_id"`
	TeamID   string `json:"team_id"`
}

// NewProvisionTask creates a new server provisioning task
func NewProvisionTask(serverID, teamID string) (*asynq.Task, error) {
	payload, err := json.Marshal(ProvisionTaskPayload{
		ServerID: serverID,
		TeamID:   teamID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeProvision, payload), nil
}

// SshKeyTaskPayload contains data for SSH key tasks
type SshKeyTaskPayload struct {
	ServerID string `json:"server_id"`
	KeyID    string `json:"key_id"`
	Action   string `json:"action"`
}

// NewSshKeyTask creates a new SSH key task (add or remove)
func NewSshKeyTask(serverID, keyID, action string) (*asynq.Task, error) {
	payload, err := json.Marshal(SshKeyTaskPayload{
		ServerID: serverID,
		KeyID:    keyID,
		Action:   action,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeSshKey, payload), nil
}
