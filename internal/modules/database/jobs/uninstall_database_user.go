package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

type UninstallDatabaseUserJob struct {
	DatabaseJobBase
	jobs.UninstallationTracker
	Payload UninstallDatabaseUserPayload
}

func (j *UninstallDatabaseUserJob) Type() string {
	return TypeUninstallDatabaseUser
}

func (j *UninstallDatabaseUserJob) Handle(ctx context.Context) error {
	j.LogInfo("Uninstalling database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, err := repository.Find[models.DatabaseUser](j.DB, ctx, j.Payload.DatabaseUserID)
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	server, err := repository.Find[servermodels.Server](j.DB, ctx, dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "uninstalling", fmt.Sprintf("Dropping database user: %s", dbUser.Name))

	factory := j.GetTaskFactory(ctx, dbUser.ServerID)
	task := factory.DropUser(tasks.DropUserConfig{
		Username:      dbUser.Name,
		AdminUser:     "root",
		AdminPassword: server.DatabasePassword.String(),
		Hosts:         []string{"%"},
	})

	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop database user: %w", err)
	}

	if !result.IsSuccessful() {
		j.LogInfo("Database user drop completed with errors", "output", result.GetOutput())
	}

	logger := activity.New(j.DB).
		WithContext(ctx).
		UseLog("database").
		On(dbUser).
		WithEvent("uninstalled")
	if j.Payload.CallerID != nil {
		logger.CausedByUser(*j.Payload.CallerID)
	}
	logger.Log("Database user was uninstalled")

	if err := j.MarkAsUninstalled(j.DB, dbUser); err != nil {
		return fmt.Errorf("failed to delete database user record: %w", err)
	}

	j.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "deleted", fmt.Sprintf("Database user %s deleted successfully", dbUser.Name))

	return nil
}

func (j *UninstallDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to uninstall database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, findErr := repository.Find[models.DatabaseUser](j.DB, ctx, j.Payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	server, findErr := repository.Find[servermodels.Server](j.DB, ctx, dbUser.ServerID)
	if findErr != nil {
		return
	}

	j.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to delete database user: %s", dbUser.Name))
}
