package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	"github.com/kkz6/launch-go/internal/pkg/traits"
)

const TypeUninstallDatabaseUser = "database:user:uninstall"

// UninstallDatabaseUserPayload holds data for database user uninstallation
type UninstallDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	CallerID       *string `json:"caller_id,omitempty"`
}

type UninstallDatabaseUserJob struct {
	pkgjobs.BaseJob[*JobContext, UninstallDatabaseUserPayload]
	traits.UninstallationTracker
}

func NewUninstallDatabaseUserJob(ctx *JobContext, payload UninstallDatabaseUserPayload) *UninstallDatabaseUserJob {
	return &UninstallDatabaseUserJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

func (j *UninstallDatabaseUserJob) Handle(ctx context.Context) error {
	j.Ctx.LogInfo("Uninstalling database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, err := repository.Find[models.DatabaseUser](ctx, j.Ctx.DB(), j.Payload.DatabaseUserID)
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	server, err := repository.Find[servermodels.Server](ctx, j.Ctx.DB(), dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "uninstalling", fmt.Sprintf("Dropping database user: %s", dbUser.Name))

	factory := j.Ctx.GetTaskFactory(ctx, dbUser.ServerID)
	task := factory.DropUser(tasks.DropUserConfig{
		Username:      dbUser.Name,
		AdminUser:     "root",
		AdminPassword: server.DatabasePassword.String(),
		Hosts:         []string{"%"},
	})

	result, err := j.Ctx.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop database user: %w", err)
	}

	if !result.IsSuccessful() {
		j.Ctx.LogInfo("Database user drop completed with errors", "output", result.GetOutput())
	}

	activity.LogWithLogPtr(ctx, j.Ctx.DB(), "database", "uninstalled", j.Payload.CallerID, dbUser, "Database user was uninstalled")

	if err := j.MarkAsUninstalled(j.Ctx.DB(), dbUser); err != nil {
		return fmt.Errorf("failed to delete database user record: %w", err)
	}

	j.Ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "deleted", fmt.Sprintf("Database user %s deleted successfully", dbUser.Name))

	return nil
}

func (j *UninstallDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to uninstall database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, findErr := repository.Find[models.DatabaseUser](ctx, j.Ctx.DB(), j.Payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	server, findErr := repository.Find[servermodels.Server](ctx, j.Ctx.DB(), dbUser.ServerID)
	if findErr != nil {
		return
	}

	j.Ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to delete database user: %s", dbUser.Name))
}

// NewUninstallDatabaseUserTask creates a database user uninstallation job
// Uses TaskID for deduplication to prevent duplicate database user uninstallations
func NewUninstallDatabaseUserTask(databaseUserID string, callerID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUninstallDatabaseUser, UninstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		CallerID:       callerID,
	}, asynq.TaskID(fmt.Sprintf("uninstall_db_user:%s", databaseUserID)))
}
