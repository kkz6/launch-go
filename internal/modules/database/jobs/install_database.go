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

type InstallDatabaseJob struct {
	DatabaseJobBase
	jobs.InstallationTracker
	Payload InstallDatabasePayload
}

func (j *InstallDatabaseJob) Type() string {
	return TypeInstallDatabase
}

func (j *InstallDatabaseJob) Handle(ctx context.Context) error {
	j.LogInfo("Installing database", "database_id", j.Payload.DatabaseID)

	database, err := repository.Find[models.Database](j.DB, ctx, j.Payload.DatabaseID)
	if err != nil {
		return fmt.Errorf("failed to find database: %w", err)
	}

	server, err := repository.Find[servermodels.Server](j.DB, ctx, database.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "installing", fmt.Sprintf("Creating database: %s", database.Name))

	factory := j.GetTaskFactory(ctx, database.ServerID)
	task := factory.CreateDatabase(tasks.CreateDatabaseConfig{
		DatabaseName:  database.Name,
		Owner:         "postgres",
		AdminUser:     "root",
		AdminPassword: server.DatabasePassword.String(),
		Charset:       "utf8mb4",
		Collation:     "utf8mb4_unicode_ci",
	})

	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to create database: %s", result.GetOutput())
	}

	if err := j.MarkAsInstalled(j.DB, database); err != nil {
		return fmt.Errorf("failed to update database status: %w", err)
	}

	j.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "installed", fmt.Sprintf("Database %s created successfully", database.Name))

	return nil
}

func (j *InstallDatabaseJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to install database", "database_id", j.Payload.DatabaseID)

	database, findErr := repository.Find[models.Database](j.DB, ctx, j.Payload.DatabaseID)
	if findErr != nil {
		return
	}

	server, findErr := repository.Find[servermodels.Server](j.DB, ctx, database.ServerID)
	if findErr != nil {
		return
	}

	j.MarkInstallationFailed(j.DB, database)

	j.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "failed", fmt.Sprintf("Failed to create database: %s", database.Name))
}
