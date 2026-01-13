package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	serverenums "github.com/kkz6/launch-go/internal/modules/server/enums"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

const TypeUninstallDatabase = "database:uninstall"

// UninstallDatabasePayload contains data for uninstalling a database
type UninstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

// UninstallDatabaseJob handles database uninstallation from a server
type UninstallDatabaseJob struct {
	jobs.BaseJob
}

// NewUninstallDatabaseJob creates a new uninstall database job handler
func NewUninstallDatabaseJob(db *gorm.DB, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, queueClient *queue.Client, logger *zerolog.Logger) *UninstallDatabaseJob {
	j := &UninstallDatabaseJob{}
	j.DB = db
	j.WS = ws
	j.Dispatcher = dispatcher
	j.Queue = queueClient
	j.Logger = logger
	return j
}

// NewUninstallDatabaseTask creates a new asynq task for uninstalling a database
func NewUninstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallDatabase, UninstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	})
}

// Handle processes the uninstall database job
func (j *UninstallDatabaseJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.ParsePayload[UninstallDatabasePayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("database_id", payload.DatabaseID).
		Msg("Uninstalling database")

	// Fetch the database
	database, err := repository.Find[models.Database](j.DB, ctx, payload.DatabaseID)
	if err != nil {
		return err
	}

	// Fetch the server
	server, err := repository.Find[servermodels.Server](j.DB, ctx, database.ServerID)
	if err != nil {
		return err
	}

	j.broadcastProgress(database.ServerID, payload.DatabaseID, "uninstalling", fmt.Sprintf("Dropping database: %s", database.Name))

	// Determine database type from installed services
	dbType := j.getDatabaseType(ctx, database.ServerID)

	// Create drop task based on database type
	var task taskrunner.Task
	if dbType == "mysql" {
		task = tasks.MySQLDropDatabase(tasks.MySQLDropDatabaseConfig{
			User:         "root",
			Password:     server.DatabasePassword.String(),
			DatabaseName: database.Name,
		})
	} else {
		task = tasks.PostgreSQLDropDatabase(tasks.PostgreSQLDropDatabaseConfig{
			DatabaseName: database.Name,
		})
	}

	// Use TaskRunner to execute on server
	result, err := servertasks.NewTaskRunner(server, task).
		WithDB(j.DB).
		WithDispatcher(j.Dispatcher).
		WithLogger(j.Logger).
		AsRoot().
		Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	if !result.IsSuccessful() {
		j.Logger.Warn().
			Str("output", result.GetOutput()).
			Msg("Database drop completed with errors")
	}

	// Delete the database record
	if err := j.DB.WithContext(ctx).Delete(database).Error; err != nil {
		return fmt.Errorf("failed to delete database record: %w", err)
	}

	j.broadcastProgress(database.ServerID, payload.DatabaseID, "deleted", fmt.Sprintf("Database %s deleted successfully", database.Name))

	return nil
}

// Failed handles job failure
func (j *UninstallDatabaseJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, parseErr := jobs.ParsePayload[UninstallDatabasePayload](t)
	if parseErr != nil {
		j.Logger.Error().Err(parseErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("database_id", payload.DatabaseID).
		Msg("Failed to uninstall database")

	// Fetch the database to get server ID for broadcasting
	database, findErr := repository.Find[models.Database](j.DB, ctx, payload.DatabaseID)
	if findErr != nil {
		return
	}

	j.broadcastProgress(database.ServerID, payload.DatabaseID, "failed", fmt.Sprintf("Failed to delete database: %s", database.Name))
}

func (j *UninstallDatabaseJob) broadcastProgress(serverID, databaseID, status, message string) {
	j.BroadcastToServer(serverID, "database.progress", map[string]interface{}{
		"server_id":   serverID,
		"database_id": databaseID,
		"status":      status,
		"message":     message,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

// getDatabaseType determines the database type from server's installed services
func (j *UninstallDatabaseJob) getDatabaseType(ctx context.Context, serverID string) string {
	service, err := repository.NewQuery[servermodels.InstalledService](j.DB, ctx).
		Where("server_id = ? AND type IN ?", serverID, []string{
			string(serverenums.ServiceTypeMySql),
			string(serverenums.ServiceTypePostgreSql),
		}).
		First()

	if err != nil {
		return "mysql" // Default to MySQL
	}

	if service.Type == serverenums.ServiceTypePostgreSql {
		return "postgresql"
	}
	return "mysql"
}
