package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

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
	pkgjobs.BaseJob[*JobContext, RemoveSSHKeyPayload]
}

// Handle processes the job
func (j *RemoveSSHKeyJob) Handle(ctx context.Context) error {
	// Find the SSH key
	sshKey, err := j.Ctx.Repos().SSHKey().FindByID(ctx, j.Payload.KeyID)
	if err != nil {
		return fmt.Errorf("failed to find SSH key: %w", err)
	}

	// Find the server
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Deauthorize the public key on the server
	task := tasks.DeauthorizePublicKey(sshKey.PublicKey, server.GetUsername())

	result, err := j.Ctx.ForServer(server).RunTask(task).
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
	activity.RecordWithLog(ctx, "server", "removed", "", sshKey, "SSH key was removed from server")

	// Detach the key from server in the database
	if err := j.Ctx.Repos().SSHKey().DetachFromServer(ctx, server.ID, sshKey.ID); err != nil {
		return fmt.Errorf("failed to detach SSH key from server: %w", err)
	}

	j.Ctx.LogInfo("SSH key removed successfully",
		"key_id", sshKey.ID,
		"server_id", server.ID,
	)

	// Broadcast event
	j.Ctx.BroadcastServerEvent(server, "ssh_key.removed", map[string]any{
		"key_id":    sshKey.ID,
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *RemoveSSHKeyJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to remove SSH key",
		"key_id", j.Payload.KeyID,
		"server_id", j.Payload.ServerID,
	)
}

func NewRemoveSSHKeyJob(ctx *JobContext, payload RemoveSSHKeyPayload) *RemoveSSHKeyJob {
	return &RemoveSSHKeyJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewRemoveSSHKeyTask creates an asynq task for removing an SSH key
// Uses TaskID for deduplication to prevent duplicate key removals
func NewRemoveSSHKeyTask(serverID, keyID string, force bool) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeRemoveSSHKey, RemoveSSHKeyPayload{
		ServerID: serverID,
		KeyID:    keyID,
		Force:    force,
	}, asynq.TaskID(fmt.Sprintf("remove_ssh_key:%s:%s", serverID, keyID)))
}
