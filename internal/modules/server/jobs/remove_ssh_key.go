package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
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
	*JobContext
}

// NewRemoveSshKeyTask creates a new asynq task for removing an SSH key
func NewRemoveSshKeyTask(serverID, sshKeyID string, isGlobal bool) (*asynq.Task, error) {
	return jobs.NewTask(TypeRemoveSshKey, RemoveSshKeyPayload{
		ServerID: serverID,
		SshKeyID: sshKeyID,
		IsGlobal: isGlobal,
	})
}

// Handle processes the remove SSH key job
func (j *RemoveSshKeyJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[RemoveSshKeyPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("ssh_key_id", payload.SshKeyID).
		Bool("is_global", payload.IsGlobal).
		Msg("Removing SSH key from server")

	// Fetch the server
	server, err := j.FindServer(ctx, payload.ServerID)
	if err != nil {
		return err
	}

	// Fetch the SSH key
	var sshKey models.SshKey
	if err := j.DB.First(&sshKey, "id = ?", payload.SshKeyID).Error; err != nil {
		return fmt.Errorf("failed to find SSH key: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "removing", "Removing SSH key...")

	// TODO: Run the actual SSH key removal task
	// _, err = j.RunTask(server, tasks.NewDeauthorizePublicKey(&sshKey)).
	//     AsRoot().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }

	_ = server // use server when implementing task execution

	// Detach the SSH key from the server
	if err := j.DB.Exec("DELETE FROM server_ssh_keys WHERE server_id = ? AND ssh_key_id = ?",
		payload.ServerID, payload.SshKeyID).Error; err != nil {
		return fmt.Errorf("failed to detach SSH key: %w", err)
	}

	// Check if the SSH key is still attached to any servers
	var count int64
	j.DB.Table("server_ssh_keys").Where("ssh_key_id = ?", payload.SshKeyID).Count(&count)

	// Delete the SSH key if it's not attached to any servers
	if count == 0 {
		if err := j.DB.Delete(&sshKey).Error; err != nil {
			j.Logger.Warn().Err(err).Msg("Failed to delete orphaned SSH key")
		}
	}

	j.broadcastProgress(payload.ServerID, "removed", "SSH key removed successfully")

	return nil
}

// Failed handles job failure
func (j *RemoveSshKeyJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[RemoveSshKeyPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("ssh_key_id", payload.SshKeyID).
		Msg("Failed to remove SSH key")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to remove SSH key")
}

func (j *RemoveSshKeyJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.ssh_key.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
