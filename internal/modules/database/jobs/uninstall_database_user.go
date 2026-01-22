package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeUninstallDatabaseUser = "database:user:uninstall"

// UninstallDatabaseUserPayload holds data for database user uninstallation
type UninstallDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	CallerID       *string `json:"caller_id,omitempty"`
}

type UninstallDatabaseUserJob struct {
	Deps    *JobDeps
	Payload UninstallDatabaseUserPayload

	dbUser *models.DatabaseUser
	server *servermodels.Server
}

func NewUninstallDatabaseUserJob(p UninstallDatabaseUserPayload) pkgjobs.Handler {
	return &UninstallDatabaseUserJob{Deps: deps, Payload: p}
}

func (j *UninstallDatabaseUserJob) Handle(ctx context.Context) error {
	j.Deps.Logger.Info().
		Str("database_user_id", j.Payload.DatabaseUserID).
		Msg("Uninstalling database user")

	var err error
	j.dbUser, err = j.Deps.Repos.User().FindByID(ctx, j.Payload.DatabaseUserID)
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	j.server, err = j.Deps.GetServer(ctx, j.dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.BroadcastUserProgress(j.server, "database_user.progress", j.Payload.DatabaseUserID, "uninstalling", fmt.Sprintf("Dropping database user: %s", j.dbUser.Name))

	factory := j.Deps.GetTaskFactory(ctx, j.dbUser.ServerID)
	task := factory.DropUser(tasks.DropUserConfig{
		Username:      j.dbUser.Name,
		AdminUser:     "root",
		AdminPassword: j.server.DatabasePassword.String(),
		Hosts:         []string{"%"},
	})

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop database user: %w", err)
	}

	if !result.IsSuccessful() {
		j.Deps.Logger.Info().
			Str("output", result.GetOutput()).
			Msg("Database user drop completed with errors")
	}

	activity.RecordWithLogPtr(ctx, "database", "uninstalled", j.Payload.CallerID, j.dbUser, "Database user was uninstalled")

	if err := j.Deps.Repos.User().Delete(ctx, j.dbUser.ID); err != nil {
		return fmt.Errorf("failed to delete database user record: %w", err)
	}

	j.Deps.BroadcastUserProgress(j.server, "database_user.progress", j.Payload.DatabaseUserID, "deleted", fmt.Sprintf("Database user %s deleted successfully", j.dbUser.Name))

	return nil
}

func (j *UninstallDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("database_user_id", j.Payload.DatabaseUserID).
		Msg("Failed to uninstall database user")

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

	j.Deps.BroadcastUserProgress(j.server, "database_user.progress", j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to delete database user: %s", j.dbUser.Name))
}

// NewUninstallDatabaseUserTask creates a database user uninstallation job
// Uses TaskID for deduplication to prevent duplicate database user uninstallations
func NewUninstallDatabaseUserTask(databaseUserID string, callerID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUninstallDatabaseUser, UninstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		CallerID:       callerID,
	}, asynq.TaskID(pkgjobs.Dedup("uninstall_db_user", databaseUserID)))
}
