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

const TypeUninstallDatabase = "database:uninstall"

// UninstallDatabasePayload holds data for database uninstallation
type UninstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

type UninstallDatabaseJob struct {
	pkgjobs.BaseJob[*JobContext, UninstallDatabasePayload]
	traits.UninstallationTracker
}

func NewUninstallDatabaseJob(ctx *JobContext, payload UninstallDatabasePayload) *UninstallDatabaseJob {
	return &UninstallDatabaseJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

func (j *UninstallDatabaseJob) Handle(ctx context.Context) error {
	j.Ctx.LogInfo("Uninstalling database", "database_id", j.Payload.DatabaseID)

	database, err := repository.Find[models.Database](ctx, j.Ctx.DB, j.Payload.DatabaseID)
	if err != nil {
		return fmt.Errorf("failed to find database: %w", err)
	}

	server, err := repository.Find[servermodels.Server](ctx, j.Ctx.DB, database.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "uninstalling", fmt.Sprintf("Dropping database: %s", database.Name))

	factory := j.Ctx.GetTaskFactory(ctx, database.ServerID)
	task := factory.DropDatabase(tasks.DropDatabaseConfig{
		DatabaseName:  database.Name,
		AdminUser:     "root",
		AdminPassword: server.DatabasePassword.String(),
	})

	result, err := j.Ctx.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	if !result.IsSuccessful() {
		j.Ctx.LogInfo("Database drop completed with errors", "output", result.GetOutput())
	}

	logger := activity.New(j.Ctx.DB).
		WithContext(ctx).
		UseLog("database").
		On(database).
		WithEvent("uninstalled")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Database was uninstalled")

	if err := j.MarkAsUninstalled(j.Ctx.DB, database); err != nil {
		return fmt.Errorf("failed to delete database record: %w", err)
	}

	j.Ctx.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "deleted", fmt.Sprintf("Database %s deleted successfully", database.Name))

	return nil
}

func (j *UninstallDatabaseJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to uninstall database", "database_id", j.Payload.DatabaseID)

	database, findErr := repository.Find[models.Database](ctx, j.Ctx.DB, j.Payload.DatabaseID)
	if findErr != nil {
		return
	}

	server, findErr := repository.Find[servermodels.Server](ctx, j.Ctx.DB, database.ServerID)
	if findErr != nil {
		return
	}

	j.Ctx.BroadcastDatabaseProgress(server, "database.progress", j.Payload.DatabaseID, "failed", fmt.Sprintf("Failed to delete database: %s", database.Name))
}

// NewUninstallDatabaseTask creates a database uninstallation job
// Uses TaskID for deduplication to prevent duplicate database uninstallations
func NewUninstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUninstallDatabase, UninstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	}, asynq.TaskID(fmt.Sprintf("uninstall_database:%s", databaseID)))
}
