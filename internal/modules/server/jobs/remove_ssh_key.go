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

const TypeRemoveSshKey = "server:remove_ssh_key"

// RemoveSshKeyPayload contains data for removing an SSH key from a server
type RemoveSshKeyPayload struct {
	ServerID string `json:"server_id"`
	SshKeyID string `json:"ssh_key_id"`
	IsGlobal bool   `json:"is_global"`
}

// RemoveSshKeyJob handles removing an SSH key from a server
type RemoveSshKeyJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewRemoveSshKeyJob creates a new remove SSH key job handler
func NewRemoveSshKeyJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *RemoveSshKeyJob {
	return &RemoveSshKeyJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewRemoveSshKeyTask creates a new asynq task for removing an SSH key
func NewRemoveSshKeyTask(serverID, sshKeyID string, isGlobal bool) (*asynq.Task, error) {
	payload, err := json.Marshal(RemoveSshKeyPayload{
		ServerID: serverID,
		SshKeyID: sshKeyID,
		IsGlobal: isGlobal,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeRemoveSshKey, payload), nil
}

// Handle processes the remove SSH key job
func (j *RemoveSshKeyJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload RemoveSshKeyPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("ssh_key_id", payload.SshKeyID).
		Bool("is_global", payload.IsGlobal).
		Msg("Removing SSH key from server")

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

	j.broadcastProgress(payload.ServerID, "removing", "Removing SSH key...")

	// TODO: Implement actual SSH key removal:
	// 1. Connect to server via SSH
	// 2. Run DeauthorizePublicKey task script
	// 3. Remove public key from authorized_keys

	// Detach the SSH key from the server
	if err := j.db.Exec("DELETE FROM server_ssh_keys WHERE server_id = ? AND ssh_key_id = ?",
		payload.ServerID, payload.SshKeyID).Error; err != nil {
		return fmt.Errorf("failed to detach SSH key: %w", err)
	}

	// Check if the SSH key is still attached to any servers
	var count int64
	j.db.Table("server_ssh_keys").Where("ssh_key_id = ?", payload.SshKeyID).Count(&count)

	// Delete the SSH key if it's not attached to any servers
	if count == 0 {
		if err := j.db.Delete(&sshKey).Error; err != nil {
			j.logger.Warn().Err(err).Msg("Failed to delete orphaned SSH key")
		}
	}

	j.broadcastProgress(payload.ServerID, "removed", "SSH key removed successfully")

	return nil
}

// Failed handles job failure
func (j *RemoveSshKeyJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload RemoveSshKeyPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("ssh_key_id", payload.SshKeyID).
		Msg("Failed to remove SSH key")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to remove SSH key")
}

func (j *RemoveSshKeyJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.ssh_key.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
