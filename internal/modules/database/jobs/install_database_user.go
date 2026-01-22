package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
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
	pkgjobs.BaseJob[*JobContext, InstallDatabaseUserPayload]
}

func NewInstallDatabaseUserJob(ctx *JobContext, payload InstallDatabaseUserPayload) *InstallDatabaseUserJob {
	return &InstallDatabaseUserJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

func (j *InstallDatabaseUserJob) Handle(ctx context.Context) error {
	j.Ctx.LogInfo("Installing database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, err := j.Ctx.Repos().User().FindByID(ctx, j.Payload.DatabaseUserID)
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	server, err := j.Ctx.GetServer(ctx, dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "installing", fmt.Sprintf("Creating database user: %s", dbUser.Name))

	factory := j.Ctx.GetTaskFactory(ctx, dbUser.ServerID)

	createUserTask := factory.CreateUser(tasks.CreateUserConfig{
		Username:      dbUser.Name,
		Password:      j.Payload.Password,
		AdminUser:     "root",
		AdminPassword: server.DatabasePassword.String(),
		Hosts:         []string{"%"},
	})

	result, err := j.Ctx.RunTaskOnServer(server, createUserTask).AsRoot().Dispatch(ctx)
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
			j.Ctx.LogError(err, "Failed to grant privileges", "database", db.Name)
		}
	}

	if err := j.Ctx.Repos().User().MarkAsInstalled(ctx, dbUser.ID); err != nil {
		return fmt.Errorf("failed to update database user status: %w", err)
	}

	activity.RecordWithLogPtr(ctx, "database", "installed", j.Payload.CallerID, dbUser, "Database user was installed")

	j.Ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "installed", fmt.Sprintf("Database user %s created successfully", dbUser.Name))

	return nil
}

func (j *InstallDatabaseUserJob) runTask(ctx context.Context, server *servermodels.Server, task taskrunner.Task) error {
	result, err := j.Ctx.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		return err
	}
	if !result.IsSuccessful() {
		j.Ctx.LogInfo("Task completed with errors", "output", result.GetOutput())
	}
	return nil
}

func (j *InstallDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to install database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, findErr := j.Ctx.Repos().User().FindByID(ctx, j.Payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	server, findErr := j.Ctx.GetServer(ctx, dbUser.ServerID)
	if findErr != nil {
		return
	}

	j.Ctx.Repos().User().MarkInstallationFailed(ctx, j.Payload.DatabaseUserID)

	j.Ctx.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to create database user: %s", dbUser.Name))
}

// NewInstallDatabaseUserTask creates a database user installation job
// Uses TaskID for deduplication to prevent duplicate database user installations
func NewInstallDatabaseUserTask(databaseUserID, password string, callerID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallDatabaseUser, InstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
	}, asynq.TaskID(fmt.Sprintf("install_db_user:%s", databaseUserID)))
}
