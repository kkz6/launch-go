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
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type InstallDatabaseUserJob struct {
	DatabaseJobBase
	jobs.InstallationTracker
	Payload InstallDatabaseUserPayload
}

func (j *InstallDatabaseUserJob) Type() string {
	return TypeInstallDatabaseUser
}

func (j *InstallDatabaseUserJob) Handle(ctx context.Context) error {
	j.LogInfo("Installing database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, err := repository.NewQuery[models.DatabaseUser](j.DB, ctx).
		WithModel("DatabaseUser").
		Preload("Databases").
		FindByID(j.Payload.DatabaseUserID).
		FirstOrFail()
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	server, err := repository.Find[servermodels.Server](j.DB, ctx, dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "installing", fmt.Sprintf("Creating database user: %s", dbUser.Name))

	factory := j.GetTaskFactory(ctx, dbUser.ServerID)

	createUserTask := factory.CreateUser(tasks.CreateUserConfig{
		Username:      dbUser.Name,
		Password:      j.Payload.Password,
		AdminUser:     "root",
		AdminPassword: server.DatabasePassword.String(),
		Hosts:         []string{"%"},
	})

	result, err := j.RunTaskOnServer(server, createUserTask).AsRoot().Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database user: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to create database user: %s", result.GetOutput())
	}

	for _, db := range dbUser.Databases {
		grantTask := factory.GrantPrivileges(tasks.GrantPrivilegesConfig{
			Username:      dbUser.Name,
			DatabaseName:  db.Name,
			AdminUser:     "root",
			AdminPassword: server.DatabasePassword.String(),
			Hosts:         []string{"%"},
		})

		if err := j.runTask(ctx, server, grantTask); err != nil {
			j.LogError(err, "Failed to grant privileges", "database", db.Name)
		}
	}

	if err := j.MarkAsInstalled(j.DB, dbUser); err != nil {
		return fmt.Errorf("failed to update database user status: %w", err)
	}

	logger := activity.New(j.DB).
		WithContext(ctx).
		UseLog("database").
		On(dbUser).
		WithEvent("installed")
	if j.Payload.CallerID != nil {
		logger.CausedByUser(*j.Payload.CallerID)
	}
	logger.Log("Database user was installed")

	j.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "installed", fmt.Sprintf("Database user %s created successfully", dbUser.Name))

	return nil
}

func (j *InstallDatabaseUserJob) runTask(ctx context.Context, server *servermodels.Server, task taskrunner.Task) error {
	result, err := j.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		return err
	}
	if !result.IsSuccessful() {
		j.LogInfo("Task completed with errors", "output", result.GetOutput())
	}
	return nil
}

func (j *InstallDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to install database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, findErr := repository.Find[models.DatabaseUser](j.DB, ctx, j.Payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	server, findErr := repository.Find[servermodels.Server](j.DB, ctx, dbUser.ServerID)
	if findErr != nil {
		return
	}

	j.MarkInstallationFailed(j.DB, dbUser)

	j.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to create database user: %s", dbUser.Name))
}
