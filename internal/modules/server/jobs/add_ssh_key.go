package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeAddSSHKey = "server:add_ssh_key"

type AddSSHKeyPayload struct {
	ServerID string `json:"server_id"`
	KeyID    string `json:"key_id"`
}

// AddSSHKeyJob adds an SSH key to a server.
// Similar to Laravel's Modules\Server\Jobs\AddSSHKeyToServer
type AddSSHKeyJob struct {
	pkgjobs.BaseJob[*JobContext, AddSSHKeyPayload]
}

// Handle processes the job
func (j *AddSSHKeyJob) Handle(ctx context.Context) error {
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

	// Authorize the public key on the server
	task := tasks.AuthorizePublicKey(sshKey.PublicKey, server.GetUsername())

	result, err := j.Ctx.ForServer(server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to add SSH key to server: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to add SSH key: %s", result.GetOutput())
	}

	// Attach the key to server in the database
	if err := j.Ctx.Repos().SSHKey().AttachToServer(ctx, server.ID, sshKey.ID); err != nil {
		return fmt.Errorf("failed to attach SSH key to server: %w", err)
	}

	// Log activity
	activity.LogWithLog(ctx, j.Ctx.DB(), "server", "added", "", sshKey, "SSH key was added to server")

	j.Ctx.LogInfo("SSH key added successfully",
		"key_id", sshKey.ID,
		"server_id", server.ID,
	)

	// Broadcast event
	j.Ctx.BroadcastServerEvent(server, "ssh_key.added", map[string]any{
		"key_id":    sshKey.ID,
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *AddSSHKeyJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to add SSH key",
		"key_id", j.Payload.KeyID,
		"server_id", j.Payload.ServerID,
	)
}

// NewAddSSHKeyJob creates a new AddSSHKeyJob with the given context and payload.
func NewAddSSHKeyJob(ctx *JobContext, payload AddSSHKeyPayload) *AddSSHKeyJob {
	return &AddSSHKeyJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewAddSSHKeyTask creates an asynq task for adding an SSH key
// Uses TaskID for deduplication to prevent duplicate key installations
func NewAddSSHKeyTask(serverID, keyID string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeAddSSHKey, AddSSHKeyPayload{
		ServerID: serverID,
		KeyID:    keyID,
	}, asynq.TaskID(fmt.Sprintf("add_ssh_key:%s:%s", serverID, keyID)))
}
