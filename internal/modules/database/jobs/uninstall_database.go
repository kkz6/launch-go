package jobs

import (
	"context"
	"encoding/json"
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
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
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
	db         *gorm.DB
	ws         *websocket.Hub
	dispatcher *taskrunner.Dispatcher
	logger     *zerolog.Logger
}

// NewUninstallDatabaseJob creates a new uninstall database job handler
func NewUninstallDatabaseJob(db *gorm.DB, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger) *UninstallDatabaseJob {
	return &UninstallDatabaseJob{
		db:         db,
		ws:         ws,
		dispatcher: dispatcher,
		logger:     logger,
	}
}

// NewUninstallDatabaseTask creates a new asynq task for uninstalling a database
func NewUninstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(UninstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeUninstallDatabase, payload), nil
}

// Handle processes the uninstall database job
func (j *UninstallDatabaseJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload UninstallDatabasePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("database_id", payload.DatabaseID).
		Msg("Uninstalling database")

	// Fetch the database
	var database models.Database
	if err := j.db.First(&database, "id = ?", payload.DatabaseID).Error; err != nil {
		return fmt.Errorf("failed to find database: %w", err)
	}

	// Fetch the server
	var server servermodels.Server
	if err := j.db.First(&server, "id = ?", database.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.broadcastProgress(database.ServerID, payload.DatabaseID, "uninstalling", fmt.Sprintf("Dropping database: %s", database.Name))

	// Determine database type from installed services
	dbType := j.getDatabaseType(database.ServerID)

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
	taskRunner := servertasks.NewTaskRunner(&server, task).
		WithDB(j.db).
		WithDispatcher(j.dispatcher).
		WithLogger(j.logger).
		AsRoot()

	result, err := taskRunner.Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	if !result.IsSuccessful() {
		j.logger.Warn().
			Str("output", result.GetOutput()).
			Msg("Database drop completed with errors")
	}

	// Delete the database record
	if err := j.db.Delete(&database).Error; err != nil {
		return fmt.Errorf("failed to delete database record: %w", err)
	}

	j.broadcastProgress(database.ServerID, payload.DatabaseID, "deleted", fmt.Sprintf("Database %s deleted successfully", database.Name))

	return nil
}

// Failed handles job failure
func (j *UninstallDatabaseJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload UninstallDatabasePayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("database_id", payload.DatabaseID).
		Msg("Failed to uninstall database")

	// Fetch the database to get server ID for broadcasting
	var database models.Database
	if findErr := j.db.First(&database, "id = ?", payload.DatabaseID).Error; findErr != nil {
		return
	}

	j.broadcastProgress(database.ServerID, payload.DatabaseID, "failed", fmt.Sprintf("Failed to delete database: %s", database.Name))
}

func (j *UninstallDatabaseJob) broadcastProgress(serverID, databaseID, status, message string) {
	j.ws.BroadcastToServer(serverID, "database.progress", map[string]interface{}{
		"server_id":   serverID,
		"database_id": databaseID,
		"status":      status,
		"message":     message,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

// getDatabaseType determines the database type from server's installed services
func (j *UninstallDatabaseJob) getDatabaseType(serverID string) string {
	var service servermodels.InstalledService
	err := j.db.Where("server_id = ? AND type IN ?", serverID, []string{
		string(serverenums.ServiceTypeMySql),
		string(serverenums.ServiceTypePostgreSql),
	}).First(&service).Error

	if err != nil {
		return "mysql" // Default to MySQL
	}

	if service.Type == serverenums.ServiceTypePostgreSql {
		return "postgresql"
	}
	return "mysql"
}
