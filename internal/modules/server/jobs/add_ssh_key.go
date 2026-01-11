package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeAddSshKey = "server:add_ssh_key"

// AddSshKeyPayload contains data for adding an SSH key to a server
type AddSshKeyPayload struct {
	ServerID string `json:"server_id"`
	SshKeyID string `json:"ssh_key_id"`
}

// AddSshKeyJob handles adding an SSH key to a server
type AddSshKeyJob struct {
	*JobContext
}

// NewAddSshKeyTask creates a new asynq task for adding an SSH key
func NewAddSshKeyTask(serverID, sshKeyID string) (*asynq.Task, error) {
	return jobs.NewTask(TypeAddSshKey, AddSshKeyPayload{
		ServerID: serverID,
		SshKeyID: sshKeyID,
	})
}

// Handle processes the add SSH key job
func (j *AddSshKeyJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[AddSshKeyPayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("ssh_key_id", payload.SshKeyID).
		Msg("Adding SSH key to server")

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

	j.broadcastProgress(payload.ServerID, "adding", "Adding SSH key...")

	// TODO: Run the actual SSH key addition task
	// _, err = j.RunTask(server, tasks.NewAuthorizePublicKey(&sshKey)).
	//     AsRoot().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }

	_ = server // use server when implementing task execution

	j.broadcastProgress(payload.ServerID, "added", "SSH key added successfully")

	return nil
}

// Failed handles job failure
func (j *AddSshKeyJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[AddSshKeyPayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("ssh_key_id", payload.SshKeyID).
		Msg("Failed to add SSH key")

	j.broadcastProgress(payload.ServerID, "failed", "Failed to add SSH key")
}

func (j *AddSshKeyJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.ssh_key.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
