package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRenameLocalUser = "server:rename_local_user"

type RenameLocalUserPayload struct {
	ServerID    string `json:"server_id"`
	OldUsername string `json:"old_username"`
	NewUsername string `json:"new_username"`
}

// RenameLocalUserJob renames the local user on a server.
type RenameLocalUserJob struct {
	ctx     *JobContext
	Payload RenameLocalUserPayload
}

// Handle processes the job
func (j *RenameLocalUserJob) Handle(ctx context.Context) error {
	// Find the server
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Create the rename user task
	task := tasks.RenameLocalUser(j.Payload.OldUsername, j.Payload.NewUsername)

	result, err := j.ctx.ForServer(server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to rename local user on server: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to rename local user: %s", result.GetOutput())
	}

	// Update the server's username in the database
	if err := j.ctx.Repos.Server().UpdateFields(ctx, server.ID, map[string]interface{}{
		"username": j.Payload.NewUsername,
	}); err != nil {
		return fmt.Errorf("failed to update server username in database: %w", err)
	}

	// Log activity
	activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(server).
		WithEvent("user_renamed").
		WithProperties(map[string]any{
			"old_username": j.Payload.OldUsername,
			"new_username": j.Payload.NewUsername,
		}).
		Log(fmt.Sprintf("Local user renamed from %s to %s", j.Payload.OldUsername, j.Payload.NewUsername))

	j.ctx.LogInfo("Local user renamed successfully",
		"server_id", server.ID,
		"old_username", j.Payload.OldUsername,
		"new_username", j.Payload.NewUsername,
	)

	// Broadcast event
	j.ctx.BroadcastServerEvent(server, "user.renamed", map[string]any{
		"server_id":    server.ID,
		"old_username": j.Payload.OldUsername,
		"new_username": j.Payload.NewUsername,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *RenameLocalUserJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to rename local user",
		"server_id", j.Payload.ServerID,
		"old_username", j.Payload.OldUsername,
		"new_username", j.Payload.NewUsername,
	)
}

// NewRenameLocalUserJob creates a new RenameLocalUserJob with the given context and payload.
func NewRenameLocalUserJob(ctx *JobContext, payload RenameLocalUserPayload) *RenameLocalUserJob {
	return &RenameLocalUserJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewRenameLocalUserTask creates an asynq task for renaming a local user
func NewRenameLocalUserTask(serverID, oldUsername, newUsername string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeRenameLocalUser, RenameLocalUserPayload{
		ServerID:    serverID,
		OldUsername: oldUsername,
		NewUsername: newUsername,
	}, asynq.TaskID(fmt.Sprintf("rename_local_user:%s", serverID)))
}
