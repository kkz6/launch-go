package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUpdateUserPublicKey = "server:update_user_public_key"

// UpdateUserPublicKeyPayload holds data for updating user's SSH public key
type UpdateUserPublicKeyPayload struct {
	ServerID  string `json:"server_id"`
	Username  string `json:"username"`
	PublicKey string `json:"public_key"`
}

// UpdateUserPublicKeyJob updates a user's SSH public key on the server
type UpdateUserPublicKeyJob struct {
	pkgjobs.BaseJob[*JobContext, UpdateUserPublicKeyPayload]
}

// NewUpdateUserPublicKeyJob creates a new UpdateUserPublicKeyJob
func NewUpdateUserPublicKeyJob(ctx *JobContext, payload UpdateUserPublicKeyPayload) *UpdateUserPublicKeyJob {
	return &UpdateUserPublicKeyJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the update user public key job
func (j *UpdateUserPublicKeyJob) Handle(ctx context.Context) error {
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Updating user public key",
		"server_id", server.ID,
		"username", j.Payload.Username,
	)

	// Create task to update the authorized_keys file
	task := tasks.UpdateAuthorizedKeys(j.Payload.Username, j.Payload.PublicKey)

	result, err := j.Ctx.ForServer(server).RunTask(task).AsRoot().Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to update user public key: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to update user public key: %s", result.GetOutput())
	}

	j.Ctx.LogInfo("User public key updated successfully",
		"server_id", server.ID,
		"username", j.Payload.Username,
	)

	j.Ctx.BroadcastServerEvent(server, "user.public_key_updated", map[string]any{
		"server_id": server.ID,
		"username":  j.Payload.Username,
	})

	return nil
}

// Failed handles job failure
func (j *UpdateUserPublicKeyJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to update user public key",
		"server_id", j.Payload.ServerID,
		"username", j.Payload.Username,
	)
}

// NewUpdateUserPublicKeyTask creates an update user public key task
func NewUpdateUserPublicKeyTask(serverID, username, publicKey string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUpdateUserPublicKey, UpdateUserPublicKeyPayload{
		ServerID:  serverID,
		Username:  username,
		PublicKey: publicKey,
	})
}
