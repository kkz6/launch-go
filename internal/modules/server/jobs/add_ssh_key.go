package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeAddSshKey = "server:add_ssh_key"

type AddSshKeyPayload struct {
	ServerID string `json:"server_id"`
	KeyID    string `json:"key_id"`
}

// AddSshKeyJob adds an SSH key to a server.
// Similar to Laravel's Modules\Server\Jobs\AddSshKeyToServer
type AddSshKeyJob struct {
	ctx     *JobContext
	Payload AddSshKeyPayload
}

// Handle processes the job
func (j *AddSshKeyJob) Handle(ctx context.Context) error {
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

	// Authorize the public key on the server
	task := tasks.AuthorizePublicKey(sshKey.PublicKey, server.GetUsername())

	result, err := j.ctx.ForServer(server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to add SSH key to server: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to add SSH key: %s", result.GetOutput())
	}

	// Attach the key to server in the database
	if err := j.ctx.Repos.SshKey().AttachToServer(ctx, server.ID, sshKey.ID); err != nil {
		return fmt.Errorf("failed to attach SSH key to server: %w", err)
	}

	// Log activity
	activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(sshKey).
		WithEvent("added").
		Log("SSH key was added to server")

	j.ctx.LogInfo("SSH key added successfully",
		"key_id", sshKey.ID,
		"server_id", server.ID,
	)

	// Broadcast event
	j.ctx.BroadcastServerEvent(server, "ssh_key.added", map[string]any{
		"key_id":    sshKey.ID,
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *AddSshKeyJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to add SSH key",
		"key_id", j.Payload.KeyID,
		"server_id", j.Payload.ServerID,
	)
}

// NewAddSshKeyJob creates a new AddSshKeyJob with the given context and payload.
func NewAddSshKeyJob(ctx *JobContext, payload AddSshKeyPayload) *AddSshKeyJob {
	return &AddSshKeyJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewAddSshKeyTask creates an asynq task for adding an SSH key
// Uses TaskID for deduplication to prevent duplicate key installations
func NewAddSshKeyTask(serverID, keyID string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeAddSshKey, AddSshKeyPayload{
		ServerID: serverID,
		KeyID:    keyID,
	}, asynq.TaskID(fmt.Sprintf("add_ssh_key:%s:%s", serverID, keyID)))
}
