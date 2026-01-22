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
	Deps    *JobDeps
	Payload InstallDatabaseUserPayload

	dbUser *models.DatabaseUser
	server *servermodels.Server
}

func NewInstallDatabaseUserJob(p InstallDatabaseUserPayload) pkgjobs.Handler {
	return &InstallDatabaseUserJob{Deps: deps, Payload: p}
}

func (j *InstallDatabaseUserJob) Handle(ctx context.Context) error {
	j.Deps.Logger.Info().
		Str("database_user_id", j.Payload.DatabaseUserID).
		Msg("Installing database user")

	var err error
	j.dbUser, err = j.Deps.Repos.User().FindByID(ctx, j.Payload.DatabaseUserID)
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	j.server, err = j.Deps.GetServer(ctx, j.dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.BroadcastUserProgress(j.server, "database_user.progress", j.Payload.DatabaseUserID, "installing", fmt.Sprintf("Creating database user: %s", j.dbUser.Name))

	factory := j.Deps.GetTaskFactory(ctx, j.dbUser.ServerID)

	createUserTask := factory.CreateUser(tasks.CreateUserConfig{
		Username:      j.dbUser.Name,
		Password:      j.Payload.Password,
		AdminUser:     "root",
		AdminPassword: j.server.DatabasePassword.String(),
		Hosts:         []string{"%"},
	})

	result, err := j.Deps.RunTask(j.server, createUserTask).AsRoot().Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database user: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to create database user: %s", result.GetOutput())
	}

	for _, db := range j.dbUser.Databases {
		grantTask := factory.GrantPrivileges(tasks.GrantPrivilegesConfig{
			Username:      j.dbUser.Name,
			DatabaseName:  db.Name,
			AdminUser:     "root",
			AdminPassword: j.server.DatabasePassword.String(),
			Hosts:         []string{"%"},
		})

		if err := j.runTask(ctx, j.server, grantTask); err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("database", db.Name).
				Msg("Failed to grant privileges")
		}
	}

	if err := j.Deps.Repos.User().MarkAsInstalled(ctx, j.dbUser.ID); err != nil {
		return fmt.Errorf("failed to update database user status: %w", err)
	}

	activity.RecordWithLogPtr(ctx, "database", "installed", j.Payload.CallerID, j.dbUser, "Database user was installed")

	j.Deps.BroadcastUserProgress(j.server, "database_user.progress", j.Payload.DatabaseUserID, "installed", fmt.Sprintf("Database user %s created successfully", j.dbUser.Name))

	return nil
}

func (j *InstallDatabaseUserJob) runTask(ctx context.Context, server *servermodels.Server, task taskrunner.Task) error {
	result, err := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		return err
	}
	if !result.IsSuccessful() {
		j.Deps.Logger.Info().
			Str("output", result.GetOutput()).
			Msg("Task completed with errors")
	}
	return nil
}

func (j *InstallDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("database_user_id", j.Payload.DatabaseUserID).
		Msg("Failed to install database user")

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

	j.Deps.Repos.User().MarkInstallationFailed(ctx, j.Payload.DatabaseUserID)

	j.Deps.BroadcastUserProgress(j.server, "database_user.progress", j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to create database user: %s", j.dbUser.Name))
}

// NewInstallDatabaseUserTask creates a database user installation job
// Uses TaskID for deduplication to prevent duplicate database user installations
func NewInstallDatabaseUserTask(databaseUserID, password string, callerID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallDatabaseUser, InstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
	}, asynq.TaskID(pkgjobs.Dedup("install_db_user", databaseUserID)))
}
