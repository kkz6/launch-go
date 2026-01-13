package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// RemoveSshKeyJob removes an SSH key from a server.
// Similar to Laravel's Modules\Server\Jobs\RemoveSshKeyFromServer
type RemoveSshKeyJob struct {
	ServerJobBase
	Payload RemoveSshKeyPayload
}

// Type returns the job type identifier
func (j *RemoveSshKeyJob) Type() string {
	return TypeRemoveSshKey
}

// Handle processes the job
func (j *RemoveSshKeyJob) Handle(ctx context.Context) error {
	// Find the SSH key
	sshKey, err := j.Repo().FindSshKeyByID(ctx, j.Payload.KeyID)
	if err != nil {
		return fmt.Errorf("failed to find SSH key: %w", err)
	}

	// Find the server
	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Deauthorize the public key on the server
	task := tasks.DeauthorizePublicKey(sshKey.PublicKey, server.GetUsername())

	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		Throw().
		Dispatch(ctx)

	if err != nil && !j.Payload.Force {
		return fmt.Errorf("failed to remove SSH key from server: %w", err)
	}

	if result != nil && !result.IsSuccessful() && !j.Payload.Force {
		return fmt.Errorf("failed to remove SSH key: %s", result.GetOutput())
	}

	// Detach the key from server in the database
	if err := j.Repo().DetachSshKeyFromServer(ctx, server.ID, sshKey.ID); err != nil {
		return fmt.Errorf("failed to detach SSH key from server: %w", err)
	}

	j.LogInfo("SSH key removed successfully",
		"key_id", sshKey.ID,
		"server_id", server.ID,
	)

	// Broadcast event
	j.BroadcastServerEvent(server.ID, "ssh_key.removed", map[string]any{
		"key_id":    sshKey.ID,
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *RemoveSshKeyJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to remove SSH key",
		"key_id", j.Payload.KeyID,
		"server_id", j.Payload.ServerID,
	)
}

// NewRemoveSshKeyTask creates an asynq task for removing an SSH key
func NewRemoveSshKeyTask(serverID, keyID string, force bool) (*asynq.Task, error) {
	return jobs.NewTask(TypeRemoveSshKey, RemoveSshKeyPayload{
		ServerID: serverID,
		KeyID:    keyID,
		Force:    force,
	})
}
