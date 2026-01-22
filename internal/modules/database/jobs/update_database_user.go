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
)

const TypeUpdateDatabaseUser = "database:user:update"

// UpdateDatabaseUserPayload holds data for database user update
type UpdateDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	Password       *string `json:"password,omitempty"`
	CallerID       *string `json:"caller_id,omitempty"`
}

type UpdateDatabaseUserJob struct {
	Deps    *JobDeps
	Payload UpdateDatabaseUserPayload

	dbUser *models.DatabaseUser
	server *servermodels.Server
}

func NewUpdateDatabaseUserJob(p UpdateDatabaseUserPayload) pkgjobs.Handler {
	return &UpdateDatabaseUserJob{Deps: deps, Payload: p}
}

func (j *UpdateDatabaseUserJob) Handle(ctx context.Context) error {
	j.Deps.Logger.Info().
		Str("database_user_id", j.Payload.DatabaseUserID).
		Msg("Updating database user")

	var err error
	j.dbUser, err = j.Deps.Repos.User().FindByID(ctx, j.Payload.DatabaseUserID)
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	j.server, err = j.Deps.GetServer(ctx, j.dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.BroadcastUserProgress(j.server, "database_user.progress", j.Payload.DatabaseUserID, "updating", fmt.Sprintf("Updating database user: %s", j.dbUser.Name))

	if j.Payload.Password != nil && *j.Payload.Password != "" {
		factory := j.Deps.GetTaskFactory(ctx, j.dbUser.ServerID)
		task := factory.UpdatePassword(tasks.UpdatePasswordConfig{
			Username:      j.dbUser.Name,
			NewPassword:   *j.Payload.Password,
			AdminUser:     "root",
			AdminPassword: j.server.DatabasePassword.String(),
			Hosts:         []string{"%"},
		})

		result, err := j.Deps.RunTask(j.server, task).
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
	if err := j.Deps.DB.WithContext(ctx).Model(j.dbUser).Update("updated_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update database user record: %w", err)
	}

	j.Deps.BroadcastUserProgress(j.server, "database_user.progress", j.Payload.DatabaseUserID, "updated", fmt.Sprintf("Database user %s updated successfully", j.dbUser.Name))

	return nil
}

func (j *UpdateDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("database_user_id", j.Payload.DatabaseUserID).
		Msg("Failed to update database user")

	if j.dbUser == nil {
		dbUser, findErr := j.Deps.Repos.User().FindByID(ctx, j.Payload.DatabaseUserID)
		if findErr != nil {
			return
		}
		j.dbUser = dbUser
	}

	if j.server == nil {
		server, findErr := j.Deps.GetServer(ctx, j.dbUser.ServerID)
		if findErr != nil {
			return
		}
		j.server = server
	}

	j.Deps.BroadcastUserProgress(j.server, "database_user.progress", j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to update database user: %s", j.dbUser.Name))
}

// NewUpdateDatabaseUserTask creates a database user update job
// Uses TaskID for deduplication to prevent duplicate database user updates
func NewUpdateDatabaseUserTask(databaseUserID string, password, callerID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUpdateDatabaseUser, UpdateDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
	}, asynq.TaskID(pkgjobs.Dedup("update_db_user", databaseUserID)))
}
