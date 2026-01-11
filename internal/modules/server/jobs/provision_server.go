package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/websocket"
)

const TypeProvisionServer = "server:provision"

// ProvisionServerPayload contains data for provisioning a server
type ProvisionServerPayload struct {
	ServerID  string   `json:"server_id"`
	TeamID    string   `json:"team_id"`
	SshKeyIDs []string `json:"ssh_key_ids,omitempty"`
}

// ProvisionServerJob handles server provisioning
type ProvisionServerJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewProvisionServerJob creates a new provision server job handler
func NewProvisionServerJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *ProvisionServerJob {
	return &ProvisionServerJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewProvisionServerTask creates a new asynq task for provisioning a server
func NewProvisionServerTask(serverID, teamID string, sshKeyIDs []string) (*asynq.Task, error) {
	payload, err := json.Marshal(ProvisionServerPayload{
		ServerID:  serverID,
		TeamID:    teamID,
		SshKeyIDs: sshKeyIDs,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeProvisionServer, payload), nil
}

// Handle processes the provision server job
func (j *ProvisionServerJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload ProvisionServerPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("team_id", payload.TeamID).
		Int("ssh_keys", len(payload.SshKeyIDs)).
		Msg("Starting server provisioning")

	// Fetch the server
	var server models.Server
	if err := j.db.First(&server, "id = ?", payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update server status to provisioning
	if err := j.db.Model(&server).Update("status", enums.ServerStatusProvisioning).Error; err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "provisioning", "Starting server provisioning...")

	// Fetch SSH keys if provided
	var sshKeys []models.SshKey
	if len(payload.SshKeyIDs) > 0 {
		if err := j.db.Where("id IN ?", payload.SshKeyIDs).Find(&sshKeys).Error; err != nil {
			j.logger.Warn().Err(err).Msg("Failed to fetch SSH keys")
		}
	}

	// TODO: Implement actual provisioning logic:
	// 1. Create server instance on cloud provider
	// 2. Wait for server to be ready
	// 3. Connect via SSH
	// 4. Run base provisioning scripts
	// 5. Install required software (PHP, database, etc.)
	// 6. Configure firewall
	// 7. Add SSH keys
	// 8. Update server status to active

	j.broadcastProgress(payload.ServerID, "active", "Server provisioned successfully")

	return nil
}

// Failed handles job failure
func (j *ProvisionServerJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload ProvisionServerPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Msg("Server provisioning failed")

	// Update server status to failed
	j.db.Model(&models.Server{}).
		Where("id = ?", payload.ServerID).
		Update("status", enums.ServerStatusFailed)

	j.broadcastProgress(payload.ServerID, "failed", "Server provisioning failed")

	// TODO: Dispatch cleanup job
}

func (j *ProvisionServerJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
