package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRemoveSshKey = "server:remove_ssh_key"

type RemoveSshKeyPayload struct {
	ServerID string `json:"server_id"`
	KeyID    string `json:"key_id"`
	Force    bool   `json:"force"`
}

// RemoveSshKeyJob removes an SSH key from a server.
// Similar to Laravel's Modules\Server\Jobs\RemoveSshKeyFromServer
type RemoveSshKeyJob struct {
	ctx     *JobContext
	Payload RemoveSshKeyPayload
}

// Handle processes the job
func (j *RemoveSshKeyJob) Handle(ctx context.Context) error {
	// Find the SSH key
	sshKey, err := j.ctx.Repos.SshKey().FindByID(ctx, j.Payload.KeyID)
	if err != nil {
		return fmt.Errorf("failed to find SSH key: %w", err)
	}

	// Find the server
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Deauthorize the public key on the server
	task := tasks.DeauthorizePublicKey(sshKey.PublicKey, server.GetUsername())

	result, err := j.ctx.ForServer(server).RunTask(task).
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
	activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(sshKey).
		WithEvent("removed").
		Log("SSH key was removed from server")

	// Detach the key from server in the database
	if err := j.ctx.Repos.SshKey().DetachFromServer(ctx, server.ID, sshKey.ID); err != nil {
		return fmt.Errorf("failed to detach SSH key from server: %w", err)
	}

	j.ctx.LogInfo("SSH key removed successfully",
		"key_id", sshKey.ID,
		"server_id", server.ID,
	)

	// Broadcast event
	j.ctx.BroadcastServerEvent(server, "ssh_key.removed", map[string]any{
		"key_id":    sshKey.ID,
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *RemoveSshKeyJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to remove SSH key",
		"key_id", j.Payload.KeyID,
		"server_id", j.Payload.ServerID,
	)
}

func NewRemoveSshKeyJob(ctx *JobContext, payload RemoveSshKeyPayload) *RemoveSshKeyJob {
	return &RemoveSshKeyJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewRemoveSshKeyTask creates an asynq task for removing an SSH key
func NewRemoveSshKeyTask(serverID, keyID string, force bool) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeRemoveSshKey, RemoveSshKeyPayload{
		ServerID: serverID,
		KeyID:    keyID,
		Force:    force,
	})
}
