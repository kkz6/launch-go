package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/database/tasks"
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
	pkgjobs.BaseJob[*JobContext, InstallDatabasePayload]
}

func NewInstallDatabaseJob(ctx *JobContext, payload InstallDatabasePayload) *InstallDatabaseJob {
	return &InstallDatabaseJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

func (j *InstallDatabaseJob) Handle(ctx context.Context) error {
	j.Ctx.LogInfo("Installing database", "database_id", j.Payload.DatabaseID)

	database, err := j.Ctx.Repos().Database().FindByID(ctx, j.Payload.DatabaseID)
	if err != nil {
		return fmt.Errorf("failed to find database: %w", err)
	}

	server, err := j.Ctx.GetServer(ctx, database.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "installing", fmt.Sprintf("Creating database: %s", database.Name))

	factory := j.Ctx.GetTaskFactory(ctx, database.ServerID)
	task := factory.CreateDatabase(tasks.CreateDatabaseConfig{
		DatabaseName:  database.Name,
		Owner:         "postgres",
		AdminUser:     "root",
		AdminPassword: server.DatabasePassword.String(),
		Charset:       "utf8mb4",
		Collation:     "utf8mb4_unicode_ci",
	})

	result, err := j.Ctx.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to create database: %s", result.GetOutput())
	}

	if err := j.Ctx.Repos().Database().MarkAsInstalled(ctx, database.ID); err != nil {
		return fmt.Errorf("failed to update database status: %w", err)
	}

	activity.RecordEventPtr(ctx, "installed", j.Payload.UserID, database, "Database was installed")

	j.Ctx.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "installed", fmt.Sprintf("Database %s created successfully", database.Name))

	return nil
}

func (j *InstallDatabaseJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to install database", "database_id", j.Payload.DatabaseID)

	database, findErr := j.Ctx.Repos().Database().FindByID(ctx, j.Payload.DatabaseID)
	if findErr != nil {
		return
	}

	server, findErr := j.Ctx.GetServer(ctx, database.ServerID)
	if findErr != nil {
		return
	}

	j.Ctx.Repos().Database().MarkInstallationFailed(ctx, j.Payload.DatabaseID)

	j.Ctx.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "failed", fmt.Sprintf("Failed to create database: %s", database.Name))
}

// NewInstallDatabaseTask creates a database installation job
// Uses TaskID for deduplication to prevent duplicate database installations
func NewInstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallDatabase, InstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	}, asynq.TaskID(fmt.Sprintf("install_database:%s", databaseID)))
}
