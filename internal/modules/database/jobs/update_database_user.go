package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

const TypeUpdateDatabaseUser = "database:user:update"

// UpdateDatabaseUserPayload holds data for database user update
type UpdateDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	Password       *string `json:"password,omitempty"`
	CallerID       *string `json:"caller_id,omitempty"`
}

type UpdateDatabaseUserJob struct {
	ctx     *JobContext
	Payload UpdateDatabaseUserPayload
}

func NewUpdateDatabaseUserJob(ctx *JobContext, payload UpdateDatabaseUserPayload) *UpdateDatabaseUserJob {
	return &UpdateDatabaseUserJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func (j *UpdateDatabaseUserJob) Handle(ctx context.Context) error {
	j.ctx.LogInfo("Updating database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, err := repository.NewQuery[models.DatabaseUser](ctx, j.ctx.DB).
		WithModel("DatabaseUser").
		Preload("Databases").
		FindByID(j.Payload.DatabaseUserID).
		FirstOrFail()
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	server, err := repository.Find[servermodels.Server](ctx, j.ctx.DB, dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "updating", fmt.Sprintf("Updating database user: %s", dbUser.Name))

	if j.Payload.Password != nil && *j.Payload.Password != "" {
		factory := j.ctx.GetTaskFactory(ctx, dbUser.ServerID)
		task := factory.UpdatePassword(tasks.UpdatePasswordConfig{
			Username:      dbUser.Name,
			NewPassword:   *j.Payload.Password,
			AdminUser:     "root",
			AdminPassword: server.DatabasePassword.String(),
			Hosts:         []string{"%"},
		})

		result, err := j.ctx.RunTaskOnServer(server, task).
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
	if err := j.ctx.DB.WithContext(ctx).Model(dbUser).Update("updated_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update database user record: %w", err)
	}

	j.ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "updated", fmt.Sprintf("Database user %s updated successfully", dbUser.Name))

	return nil
}

func (j *UpdateDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to update database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, findErr := repository.Find[models.DatabaseUser](ctx, j.ctx.DB, j.Payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	server, findErr := repository.Find[servermodels.Server](ctx, j.ctx.DB, dbUser.ServerID)
	if findErr != nil {
		return
	}

	j.ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to update database user: %s", dbUser.Name))
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
