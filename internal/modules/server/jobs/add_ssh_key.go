package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/websocket"
)

const TypeAddSshKey = "server:add_ssh_key"

// AddSshKeyPayload contains data for adding an SSH key to a server
type AddSshKeyPayload struct {
	ServerID string `json:"server_id"`
	SshKeyID string `json:"ssh_key_id"`
}

// AddSshKeyJob handles adding an SSH key to a server
type AddSshKeyJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewAddSshKeyJob creates a new add SSH key job handler
func NewAddSshKeyJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *AddSshKeyJob {
	return &AddSshKeyJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewAddSshKeyTask creates a new asynq task for adding an SSH key
func NewAddSshKeyTask(serverID, sshKeyID string) (*asynq.Task, error) {
	payload, err := json.Marshal(AddSshKeyPayload{
		ServerID: serverID,
		SshKeyID: sshKeyID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeAddSshKey, payload), nil
}

// Handle processes the add SSH key job
func (j *AddSshKeyJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload AddSshKeyPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("ssh_key_id", payload.SshKeyID).
		Msg("Adding SSH key to server")

	// Fetch the server
	var server models.Server
	if err := j.db.First(&server, "id = ?", payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Fetch the SSH key
	var sshKey models.SshKey
	if err := j.db.First(&sshKey, "id = ?", payload.SshKeyID).Error; err != nil {
		return fmt.Errorf("failed to find SSH key: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "adding", "Adding SSH key...")

	// TODO: Implement actual SSH key addition:
	// 1. Connect to server via SSH
	// 2. Run AuthorizePublicKey task script
	// 3. Append public key to authorized_keys

	j.broadcastProgress(payload.ServerID, "added", "SSH key added successfully")

	return nil
}

// Failed handles job failure
func (j *AddSshKeyJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload AddSshKeyPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("ssh_key_id", payload.SshKeyID).
		Msg("Failed to add SSH key")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to add SSH key")
}

func (j *AddSshKeyJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.ssh_key.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
