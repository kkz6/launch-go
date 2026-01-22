package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUpdateDatabaseUser = "database:user:update"

// UpdateDatabaseUserPayload holds data for database user update
type UpdateDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	Password       *string `json:"password,omitempty"`
	CallerID       *string `json:"caller_id,omitempty"`
}

type UpdateDatabaseUserJob struct {
	pkgjobs.BaseJob[*JobContext, UpdateDatabaseUserPayload]
}

func NewUpdateDatabaseUserJob(ctx *JobContext, payload UpdateDatabaseUserPayload) *UpdateDatabaseUserJob {
	return &UpdateDatabaseUserJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

func (j *UpdateDatabaseUserJob) Handle(ctx context.Context) error {
	j.Ctx.LogInfo("Updating database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, err := j.Ctx.Repos().User().FindByID(ctx, j.Payload.DatabaseUserID)
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	server, err := j.Ctx.GetServer(ctx, dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "updating", fmt.Sprintf("Updating database user: %s", dbUser.Name))

	if j.Payload.Password != nil && *j.Payload.Password != "" {
		factory := j.Ctx.GetTaskFactory(ctx, dbUser.ServerID)
		task := factory.UpdatePassword(tasks.UpdatePasswordConfig{
			Username:      dbUser.Name,
			NewPassword:   *j.Payload.Password,
			AdminUser:     "root",
			AdminPassword: server.DatabasePassword.String(),
			Hosts:         []string{"%"},
		})

		result, err := j.Ctx.RunTaskOnServer(server, task).
			AsRoot().
			Dispatch(ctx)
		if err != nil {
			return fmt.Errorf("failed to update database user password: %w", err)
		}

		if !result.IsSuccessful() {
			return fmt.Errorf("failed to update database user password: %s", result.GetOutput())
		}
	}

	now := time.Now()
	if err := j.Ctx.DB().WithContext(ctx).Model(dbUser).Update("updated_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update database user record: %w", err)
	}

	j.Ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "updated", fmt.Sprintf("Database user %s updated successfully", dbUser.Name))

	return nil
}

func (j *UpdateDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to update database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, findErr := j.Ctx.Repos().User().FindByID(ctx, j.Payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	server, findErr := j.Ctx.GetServer(ctx, dbUser.ServerID)
	if findErr != nil {
		return
	}

	j.Ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to update database user: %s", dbUser.Name))
}

// NewUpdateDatabaseUserTask creates a database user update job
// Uses TaskID for deduplication to prevent duplicate database user updates
func NewUpdateDatabaseUserTask(databaseUserID string, password, callerID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUpdateDatabaseUser, UpdateDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
	}, asynq.TaskID(fmt.Sprintf("update_db_user:%s", databaseUserID)))
}
