package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

type UninstallDatabaseJob struct {
	DatabaseJobBase
	jobs.UninstallationTracker
	Payload UninstallDatabasePayload
}

func (j *UninstallDatabaseJob) Type() string {
	return TypeUninstallDatabase
}

func (j *UninstallDatabaseJob) Handle(ctx context.Context) error {
	j.LogInfo("Uninstalling database", "database_id", j.Payload.DatabaseID)

	database, err := repository.Find[models.Database](j.DB, ctx, j.Payload.DatabaseID)
	if err != nil {
		return fmt.Errorf("failed to find database: %w", err)
	}

	server, err := repository.Find[servermodels.Server](j.DB, ctx, database.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "uninstalling", fmt.Sprintf("Dropping database: %s", database.Name))

	factory := j.GetTaskFactory(ctx, database.ServerID)
	task := factory.DropDatabase(tasks.DropDatabaseConfig{
		DatabaseName:  database.Name,
		AdminUser:     "root",
		AdminPassword: server.DatabasePassword.String(),
	})

	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	if !result.IsSuccessful() {
		j.LogInfo("Database drop completed with errors", "output", result.GetOutput())
	}

	if err := j.MarkAsUninstalled(j.DB, database); err != nil {
		return fmt.Errorf("failed to delete database record: %w", err)
	}

	j.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "deleted", fmt.Sprintf("Database %s deleted successfully", database.Name))

	return nil
}

func (j *UninstallDatabaseJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to uninstall database", "database_id", j.Payload.DatabaseID)

	database, findErr := repository.Find[models.Database](j.DB, ctx, j.Payload.DatabaseID)
	if findErr != nil {
		return
	}

	server, findErr := repository.Find[servermodels.Server](j.DB, ctx, database.ServerID)
	if findErr != nil {
		return
	}

	j.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "failed", fmt.Sprintf("Failed to delete database: %s", database.Name))
}
