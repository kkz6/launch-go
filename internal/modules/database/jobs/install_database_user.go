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
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const TypeInstallDatabaseUser = "database:user:install"

// InstallDatabaseUserPayload holds data for database user installation
type InstallDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	Password       string  `json:"password"`
	CallerID       *string `json:"caller_id,omitempty"`
}

type InstallDatabaseUserJob struct {
	ctx *JobContext
	pkgjobs.InstallationTracker
	Payload InstallDatabaseUserPayload
}

func NewInstallDatabaseUserJob(ctx *JobContext, payload InstallDatabaseUserPayload) *InstallDatabaseUserJob {
	return &InstallDatabaseUserJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func (j *InstallDatabaseUserJob) Handle(ctx context.Context) error {
	j.ctx.LogInfo("Installing database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, err := repository.NewQuery[models.DatabaseUser](j.ctx.DB, ctx).
		WithModel("DatabaseUser").
		Preload("Databases").
		FindByID(j.Payload.DatabaseUserID).
		FirstOrFail()
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	server, err := repository.Find[servermodels.Server](j.ctx.DB, ctx, dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "installing", fmt.Sprintf("Creating database user: %s", dbUser.Name))

	factory := j.ctx.GetTaskFactory(ctx, dbUser.ServerID)

	createUserTask := factory.CreateUser(tasks.CreateUserConfig{
		Username:      dbUser.Name,
		Password:      j.Payload.Password,
		AdminUser:     "root",
		AdminPassword: server.DatabasePassword.String(),
		Hosts:         []string{"%"},
	})

	result, err := j.ctx.RunTaskOnServer(server, createUserTask).AsRoot().Dispatch(ctx)
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
			j.ctx.LogError(err, "Failed to grant privileges", "database", db.Name)
		}
	}

	if err := j.MarkAsInstalled(j.ctx.DB, dbUser); err != nil {
		return fmt.Errorf("failed to update database user status: %w", err)
	}

	logger := activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("database").
		On(dbUser).
		WithEvent("installed")
	if j.Payload.CallerID != nil {
		logger.CausedByUser(*j.Payload.CallerID)
	}
	logger.Log("Database user was installed")

	j.ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "installed", fmt.Sprintf("Database user %s created successfully", dbUser.Name))

	return nil
}

func (j *InstallDatabaseUserJob) runTask(ctx context.Context, server *servermodels.Server, task taskrunner.Task) error {
	result, err := j.ctx.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		return err
	}
	if !result.IsSuccessful() {
		j.ctx.LogInfo("Task completed with errors", "output", result.GetOutput())
	}
	return nil
}

func (j *InstallDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to install database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, findErr := repository.Find[models.DatabaseUser](j.ctx.DB, ctx, j.Payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	server, findErr := repository.Find[servermodels.Server](j.ctx.DB, ctx, dbUser.ServerID)
	if findErr != nil {
		return
	}

	j.MarkInstallationFailed(j.ctx.DB, dbUser)

	j.ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to create database user: %s", dbUser.Name))
}

// NewInstallDatabaseUserTask creates a database user installation job
func NewInstallDatabaseUserTask(databaseUserID, password string, callerID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallDatabaseUser, InstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
	})
}
