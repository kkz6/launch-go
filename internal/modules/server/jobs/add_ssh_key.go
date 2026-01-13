package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// AddSshKeyJob adds an SSH key to a server.
// Similar to Laravel's Modules\Server\Jobs\AddSshKeyToServer
type AddSshKeyJob struct {
	ServerJobBase
	Payload AddSshKeyPayload
}

// Type returns the job type identifier
func (j *AddSshKeyJob) Type() string {
	return TypeAddSshKey
}

// Handle processes the job
func (j *AddSshKeyJob) Handle(ctx context.Context) error {
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

	// Authorize the public key on the server
	task := tasks.AuthorizePublicKey(sshKey.PublicKey, server.GetUsername())

	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to add SSH key to server: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to add SSH key: %s", result.GetOutput())
	}

	// Attach the key to server in the database
	if err := j.Repo().AttachSshKeyToServer(ctx, server.ID, sshKey.ID); err != nil {
		return fmt.Errorf("failed to attach SSH key to server: %w", err)
	}

	j.LogInfo("SSH key added successfully",
		"key_id", sshKey.ID,
		"server_id", server.ID,
	)

	// Broadcast event
	j.BroadcastServerEvent(server.ID, "ssh_key.added", map[string]interface{}{
		"key_id":    sshKey.ID,
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *AddSshKeyJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to add SSH key",
		"key_id", j.Payload.KeyID,
		"server_id", j.Payload.ServerID,
	)
}

// NewAddSshKeyTask creates an asynq task for adding an SSH key
func NewAddSshKeyTask(serverID, keyID string) (*asynq.Task, error) {
	return jobs.NewTask(TypeAddSshKey, AddSshKeyPayload{
		ServerID: serverID,
		KeyID:    keyID,
	})
}
