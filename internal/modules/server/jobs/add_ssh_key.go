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

const TypeAddSSHKey = "server:add_ssh_key"

type AddSSHKeyPayload struct {
	ServerID string `json:"server_id"`
	KeyID    string `json:"key_id"`
}

// AddSSHKeyJob adds an SSH key to a server.
// Similar to Laravel's Modules\Server\Jobs\AddSSHKeyToServer
type AddSSHKeyJob struct {
	Deps    *JobDeps
	Payload AddSSHKeyPayload

	server *models.Server
	sshKey *models.SSHKey
}

func NewAddSSHKeyJob(p AddSSHKeyPayload) pkgjobs.Handler {
	return &AddSSHKeyJob{Deps: deps, Payload: p}
}

func (j *AddSSHKeyJob) Handle(ctx context.Context) error {
	var err error

	j.sshKey, err = j.Deps.Repos.SSHKey().FindByID(ctx, j.Payload.KeyID)
	if err != nil {
		return fmt.Errorf("find SSH key: %w", err)
	}

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	task := tasks.AuthorizePublicKey(j.sshKey.PublicKey, j.server.GetUsername())

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("add SSH key to server: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("add SSH key: %s", result.GetOutput())
	}

	if err := j.Deps.Repos.SSHKey().AttachToServer(ctx, j.server.ID, j.sshKey.ID); err != nil {
		return fmt.Errorf("attach SSH key to server: %w", err)
	}

	activity.RecordWithLog(ctx, "server", "added", "", j.sshKey, "SSH key was added to server")

	j.Deps.Logger.Info().
		Str("key_id", j.sshKey.ID).
		Str("server_id", j.server.ID).
		Msg("SSH key added successfully")

	j.Deps.BroadcastServerEvent(j.server, "ssh_key.added", map[string]any{
		"key_id":    j.sshKey.ID,
		"server_id": j.server.ID,
	})

	return nil
}

func (j *AddSSHKeyJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("key_id", j.Payload.KeyID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to add SSH key")
}

func NewAddSSHKeyTask(serverID, keyID string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeAddSSHKey,
		AddSSHKeyPayload{ServerID: serverID, KeyID: keyID},
		pkgjobs.Dedup("add_ssh_key", serverID, keyID),
	)
}
