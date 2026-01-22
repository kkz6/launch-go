package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
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
	Deps    *JobDeps
	Payload UpdateUserPublicKeyPayload

	server *models.Server
}

func NewUpdateUserPublicKeyJob(p UpdateUserPublicKeyPayload) pkgjobs.Handler {
	return &UpdateUserPublicKeyJob{Deps: deps, Payload: p}
}

// Handle executes the update user public key job
func (j *UpdateUserPublicKeyJob) Handle(ctx context.Context) error {
	var err error
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("username", j.Payload.Username).
		Msg("Updating user public key")

	// Create task to update the authorized_keys file
	task := tasks.UpdateAuthorizedKeys(j.Payload.Username, j.Payload.PublicKey)

	result, err := j.Deps.RunTask(j.server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to update user public key: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to update user public key: %s", result.GetOutput())
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("username", j.Payload.Username).
		Msg("User public key updated successfully")

	j.Deps.BroadcastServerEvent(j.server, "user.public_key_updated", map[string]any{
		"server_id": j.server.ID,
		"username":  j.Payload.Username,
	})

	return nil
}

// Failed handles job failure
func (j *UpdateUserPublicKeyJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("username", j.Payload.Username).
		Msg("Failed to update user public key")
}

// NewUpdateUserPublicKeyTask creates an update user public key task
func NewUpdateUserPublicKeyTask(serverID, username, publicKey string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUpdateUserPublicKey, UpdateUserPublicKeyPayload{
		ServerID:  serverID,
		Username:  username,
		PublicKey: publicKey,
	})
}
