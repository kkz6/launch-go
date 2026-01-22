package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeRemoveSSHKey = "server:remove_ssh_key"

type RemoveSSHKeyPayload struct {
	ServerID string `json:"server_id"`
	KeyID    string `json:"key_id"`
	Force    bool   `json:"force"`
}

// RemoveSSHKeyJob removes an SSH key from a server.
// Similar to Laravel's Modules\Server\Jobs\RemoveSSHKeyFromServer
type RemoveSSHKeyJob struct {
	Deps    *JobDeps
	Payload RemoveSSHKeyPayload

	server *models.Server
	sshKey *models.SSHKey
}

func NewRemoveSSHKeyJob(p RemoveSSHKeyPayload) pkgjobs.Handler {
	return &RemoveSSHKeyJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *RemoveSSHKeyJob) Handle(ctx context.Context) error {
	var err error

	// Find the SSH key
	j.sshKey, err = j.Deps.Repos.SSHKey().FindByID(ctx, j.Payload.KeyID)
	if err != nil {
		return fmt.Errorf("failed to find SSH key: %w", err)
	}

	// Find the server
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Deauthorize the public key on the server
	task := tasks.DeauthorizePublicKey(j.sshKey.PublicKey, j.server.GetUsername())

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		Throw().
		Dispatch(ctx)

	if err != nil && !j.Payload.Force {
		return fmt.Errorf("failed to remove SSH key from server: %w", err)
	}

	if result != nil && !result.IsSuccessful() && !j.Payload.Force {
		return fmt.Errorf("failed to remove SSH key: %s", result.GetOutput())
	}

	// Log activity before detaching
	activity.RecordWithLog(ctx, "server", "removed", "", j.sshKey, "SSH key was removed from server")

	// Detach the key from server in the database
	if err := j.Deps.Repos.SSHKey().DetachFromServer(ctx, j.server.ID, j.sshKey.ID); err != nil {
		return fmt.Errorf("failed to detach SSH key from server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("key_id", j.sshKey.ID).
		Str("server_id", j.server.ID).
		Msg("SSH key removed successfully")

	// Broadcast event
	j.Deps.BroadcastServerEvent(j.server, "ssh_key.removed", map[string]any{
		"key_id":    j.sshKey.ID,
		"server_id": j.server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *RemoveSSHKeyJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("key_id", j.Payload.KeyID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to remove SSH key")
}

// NewRemoveSSHKeyTask creates an asynq task for removing an SSH key
// Uses TaskID for deduplication to prevent duplicate key removals
func NewRemoveSSHKeyTask(serverID, keyID string, force bool) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeRemoveSSHKey,
		RemoveSSHKeyPayload{
			ServerID: serverID,
			KeyID:    keyID,
			Force:    force,
		},
		pkgjobs.Dedup("remove_ssh_key", serverID, keyID),
	)
}
