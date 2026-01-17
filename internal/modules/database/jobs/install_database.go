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

const TypeInstallDatabase = "database:install"

// InstallDatabasePayload holds data for database installation
type InstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

type InstallDatabaseJob struct {
	ctx *JobContext
	traits.InstallationTracker
	Payload InstallDatabasePayload
}

func NewInstallDatabaseJob(ctx *JobContext, payload InstallDatabasePayload) *InstallDatabaseJob {
	return &InstallDatabaseJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func (j *InstallDatabaseJob) Handle(ctx context.Context) error {
	j.ctx.LogInfo("Installing database", "database_id", j.Payload.DatabaseID)

	database, err := repository.Find[models.Database](j.ctx.DB, ctx, j.Payload.DatabaseID)
	if err != nil {
		return fmt.Errorf("failed to find database: %w", err)
	}

	server, err := repository.Find[servermodels.Server](j.ctx.DB, ctx, database.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "installing", fmt.Sprintf("Creating database: %s", database.Name))

	factory := j.ctx.GetTaskFactory(ctx, database.ServerID)
	task := factory.CreateDatabase(tasks.CreateDatabaseConfig{
		DatabaseName:  database.Name,
		Owner:         "postgres",
		AdminUser:     "root",
		AdminPassword: server.DatabasePassword.String(),
		Charset:       "utf8mb4",
		Collation:     "utf8mb4_unicode_ci",
	})

	result, err := j.ctx.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to create database: %s", result.GetOutput())
	}

	if err := j.MarkAsInstalled(j.ctx.DB, database); err != nil {
		return fmt.Errorf("failed to update database status: %w", err)
	}

	logger := activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("database").
		On(database).
		WithEvent("installed")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Database was installed")

	j.ctx.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "installed", fmt.Sprintf("Database %s created successfully", database.Name))

	return nil
}

func (j *InstallDatabaseJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to install database", "database_id", j.Payload.DatabaseID)

	database, findErr := repository.Find[models.Database](j.ctx.DB, ctx, j.Payload.DatabaseID)
	if findErr != nil {
		return
	}

	server, findErr := repository.Find[servermodels.Server](j.ctx.DB, ctx, database.ServerID)
	if findErr != nil {
		return
	}

	j.MarkInstallationFailed(j.ctx.DB, database)

	j.ctx.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "failed", fmt.Sprintf("Failed to create database: %s", database.Name))
}

// NewInstallDatabaseTask creates a database installation job
// Uses TaskID for deduplication to prevent duplicate database installations
func NewInstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallDatabase, InstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	}, asynq.TaskID(fmt.Sprintf("install_database:%s", databaseID)))
}
