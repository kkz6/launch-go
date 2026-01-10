package jobs

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/websocket"
)

// Task types
const (
	TypeProvision         = "server:provision"
	TypeInstallPHP        = "server:install_php"
	TypeConfigureFirewall = "server:configure_firewall"
	TypeInstallDatabase   = "server:install_database"
	TypeReboot            = "server:reboot"
)

type Handler struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

func NewHandler(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *Handler {
	return &Handler{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// Provision Task
type ProvisionPayload struct {
	ServerID string `json:"server_id"`
	TeamID   string `json:"team_id"`
}

func NewProvisionTask(serverID, teamID string) (*asynq.Task, error) {
	payload, err := json.Marshal(ProvisionPayload{
		ServerID: serverID,
		TeamID:   teamID,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeProvision, payload), nil
}

func (h *Handler) HandleProvision(ctx context.Context, t *asynq.Task) error {
	var payload ProvisionPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("server_id", payload.ServerID).Msg("Starting server provision")

	// Broadcast progress
	h.broadcastProgress(payload.ServerID, "provisioning", "Creating server instance...")

	// TODO: Implement actual provisioning logic
	// 1. Create server on cloud provider
	// 2. Wait for server to be ready
	// 3. Connect via SSH
	// 4. Run provisioning scripts
	// 5. Install PHP, web server, database
	// 6. Configure firewall
	// 7. Update server status

	h.broadcastProgress(payload.ServerID, "active", "Server provisioned successfully")

	return nil
}

// Install PHP Task
type InstallPHPPayload struct {
	ServerID   string `json:"server_id"`
	PHPVersion string `json:"php_version"`
}

func NewInstallPHPTask(serverID, phpVersion string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallPHPPayload{
		ServerID:   serverID,
		PHPVersion: phpVersion,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeInstallPHP, payload), nil
}

func (h *Handler) HandleInstallPHP(ctx context.Context, t *asynq.Task) error {
	var payload InstallPHPPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().
		Str("server_id", payload.ServerID).
		Str("php_version", payload.PHPVersion).
		Msg("Installing PHP version")

	// TODO: Implement PHP installation

	return nil
}

// Configure Firewall Task
type ConfigureFirewallPayload struct {
	ServerID string `json:"server_id"`
}

func NewConfigureFirewallTask(serverID string) (*asynq.Task, error) {
	payload, err := json.Marshal(ConfigureFirewallPayload{ServerID: serverID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeConfigureFirewall, payload), nil
}

func (h *Handler) HandleConfigureFirewall(ctx context.Context, t *asynq.Task) error {
	var payload ConfigureFirewallPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("server_id", payload.ServerID).Msg("Configuring firewall")

	// TODO: Implement firewall configuration

	return nil
}

// Install Database Task
type InstallDatabasePayload struct {
	ServerID     string `json:"server_id"`
	DatabaseType string `json:"database_type"`
}

func NewInstallDatabaseTask(serverID, databaseType string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallDatabasePayload{
		ServerID:     serverID,
		DatabaseType: databaseType,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeInstallDatabase, payload), nil
}

func (h *Handler) HandleInstallDatabase(ctx context.Context, t *asynq.Task) error {
	var payload InstallDatabasePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().
		Str("server_id", payload.ServerID).
		Str("database_type", payload.DatabaseType).
		Msg("Installing database")

	// TODO: Implement database installation

	return nil
}

// Reboot Task
type RebootPayload struct {
	ServerID string `json:"server_id"`
}

func NewRebootTask(serverID string) (*asynq.Task, error) {
	payload, err := json.Marshal(RebootPayload{ServerID: serverID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeReboot, payload), nil
}

func (h *Handler) HandleReboot(ctx context.Context, t *asynq.Task) error {
	var payload RebootPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("server_id", payload.ServerID).Msg("Rebooting server")

	// TODO: Implement server reboot

	return nil
}

// Helper methods
func (h *Handler) broadcastProgress(serverID, status, message string) {
	h.ws.BroadcastToServer(serverID, "server.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
