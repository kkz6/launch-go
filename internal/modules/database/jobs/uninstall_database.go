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

const TypeUninstallDatabase = "database:uninstall"

// UninstallDatabasePayload holds data for database uninstallation
type UninstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

type UninstallDatabaseJob struct {
	Deps    *JobDeps
	Payload UninstallDatabasePayload

	database *models.Database
	server   *servermodels.Server
}

func NewUninstallDatabaseJob(p UninstallDatabasePayload) pkgjobs.Handler {
	return &UninstallDatabaseJob{Deps: deps, Payload: p}
}

func (j *UninstallDatabaseJob) Handle(ctx context.Context) error {
	j.Deps.Logger.Info().
		Str("database_id", j.Payload.DatabaseID).
		Msg("Uninstalling database")

	var err error
	j.database, err = j.Deps.Repos.Database().FindByID(ctx, j.Payload.DatabaseID)
	if err != nil {
		return fmt.Errorf("failed to find database: %w", err)
	}

	j.server, err = j.Deps.GetServer(ctx, j.database.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.BroadcastDatabaseProgress(j.server, "database.progress", j.Payload.DatabaseID, "uninstalling", fmt.Sprintf("Dropping database: %s", j.database.Name))

	factory := j.Deps.GetTaskFactory(ctx, j.database.ServerID)
	task := factory.DropDatabase(tasks.DropDatabaseConfig{
		DatabaseName:  j.database.Name,
		AdminUser:     "root",
		AdminPassword: j.server.DatabasePassword.String(),
	})

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	if !result.IsSuccessful() {
		j.Deps.Logger.Info().
			Str("output", result.GetOutput()).
			Msg("Database drop completed with errors")
	}

	activity.RecordEventPtr(ctx, "uninstalled", j.Payload.UserID, j.database, "Database was uninstalled")

	if err := j.Deps.Repos.Database().Delete(ctx, j.database.ID); err != nil {
		return fmt.Errorf("failed to delete database record: %w", err)
	}

	j.Deps.BroadcastDatabaseProgress(j.server, "database.progress", j.Payload.DatabaseID, "deleted", fmt.Sprintf("Database %s deleted successfully", j.database.Name))

	return nil
}

func (j *UninstallDatabaseJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("database_id", j.Payload.DatabaseID).
		Msg("Failed to uninstall database")

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

	j.Deps.BroadcastDatabaseProgress(j.server, "database.progress", j.Payload.DatabaseID, "failed", fmt.Sprintf("Failed to delete database: %s", j.database.Name))
}

// NewUninstallDatabaseTask creates a database uninstallation job
// Uses TaskID for deduplication to prevent duplicate database uninstallations
func NewUninstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUninstallDatabase, UninstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	}, asynq.TaskID(pkgjobs.Dedup("uninstall_database", databaseID)))
}
