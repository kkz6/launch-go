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

const TypeInstallDatabase = "database:install"

// InstallDatabasePayload holds data for database installation
type InstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

type InstallDatabaseJob struct {
	Deps    *JobDeps
	Payload InstallDatabasePayload

	database *models.Database
	server   *servermodels.Server
}

func NewInstallDatabaseJob(p InstallDatabasePayload) pkgjobs.Handler {
	return &InstallDatabaseJob{Deps: deps, Payload: p}
}

func (j *InstallDatabaseJob) Handle(ctx context.Context) error {
	j.Deps.Logger.Info().
		Str("database_id", j.Payload.DatabaseID).
		Msg("Installing database")

	var err error
	j.database, err = j.Deps.Repos.Database().FindByID(ctx, j.Payload.DatabaseID)
	if err != nil {
		return fmt.Errorf("failed to find database: %w", err)
	}

	j.server, err = j.Deps.GetServer(ctx, j.database.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.BroadcastDatabaseProgress(j.server, "database.progress", j.Payload.DatabaseID, "installing", fmt.Sprintf("Creating database: %s", j.database.Name))

	factory := j.Deps.GetTaskFactory(ctx, j.database.ServerID)
	task := factory.CreateDatabase(tasks.CreateDatabaseConfig{
		DatabaseName:  j.database.Name,
		Owner:         "postgres",
		AdminUser:     "root",
		AdminPassword: j.server.DatabasePassword.String(),
		Charset:       "utf8mb4",
		Collation:     "utf8mb4_unicode_ci",
	})

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to create database: %s", result.GetOutput())
	}

	if err := j.Deps.Repos.Database().MarkAsInstalled(ctx, j.database.ID); err != nil {
		return fmt.Errorf("failed to update database status: %w", err)
	}

	activity.RecordEventPtr(ctx, "installed", j.Payload.UserID, j.database, "Database was installed")

	j.Deps.BroadcastDatabaseProgress(j.server, "database.progress", j.Payload.DatabaseID, "installed", fmt.Sprintf("Database %s created successfully", j.database.Name))

	return nil
}

func (j *InstallDatabaseJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("database_id", j.Payload.DatabaseID).
		Msg("Failed to install database")

	if j.database == nil {
		database, findErr := j.Deps.Repos.Database().FindByID(ctx, j.Payload.DatabaseID)
		if findErr != nil {
			return
		}
		j.database = database
	}

	if j.server == nil {
		server, findErr := j.Deps.GetServer(ctx, j.database.ServerID)
		if findErr != nil {
			return
		}
		j.server = server
	}

	if markErr := j.Deps.Repos.Database().MarkInstallationFailed(ctx, j.Payload.DatabaseID); markErr != nil {
		j.Deps.Logger.Error().Err(markErr).
			Str("database_id", j.Payload.DatabaseID).
			Msg("Failed to mark database installation as failed")
	}

	j.Deps.BroadcastDatabaseProgress(j.server, "database.progress", j.Payload.DatabaseID, "failed", fmt.Sprintf("Failed to create database: %s", j.database.Name))
}

// NewInstallDatabaseTask creates a database installation job
// Uses TaskID for deduplication to prevent duplicate database installations
func NewInstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallDatabase, InstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	}, asynq.TaskID(pkgjobs.Dedup("install_database", databaseID)))
}
