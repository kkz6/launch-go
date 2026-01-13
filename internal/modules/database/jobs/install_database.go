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

const TypeInstallDatabase = "database:install"

// InstallDatabasePayload contains data for installing a database
type InstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

// InstallDatabaseJob handles database installation on a server
type InstallDatabaseJob struct {
	db         *gorm.DB
	ws         *websocket.Hub
	dispatcher *taskrunner.Dispatcher
	logger     *zerolog.Logger
}

// NewInstallDatabaseJob creates a new install database job handler
func NewInstallDatabaseJob(db *gorm.DB, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger) *InstallDatabaseJob {
	return &InstallDatabaseJob{
		db:         db,
		ws:         ws,
		dispatcher: dispatcher,
		logger:     logger,
	}
}

// NewInstallDatabaseTask creates a new asynq task for installing a database
func NewInstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeInstallDatabase, payload), nil
}

// Handle processes the install database job
func (j *InstallDatabaseJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload InstallDatabasePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("database_id", payload.DatabaseID).
		Msg("Installing database")

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

	j.broadcastProgress(database.ServerID, payload.DatabaseID, "installing", fmt.Sprintf("Creating database: %s", database.Name))

	// Determine database type from installed services
	dbType := j.getDatabaseType(database.ServerID)

	// Create task based on database type
	var task taskrunner.Task
	if dbType == "mysql" {
		task = tasks.MySQLCreateDatabase(tasks.MySQLCreateDatabaseConfig{
			User:         "root",
			Password:     server.DatabasePassword.String(),
			DatabaseName: database.Name,
			Charset:      "utf8mb4",
			Collation:    "utf8mb4_unicode_ci",
		})
	} else {
		task = tasks.PostgreSQLCreateDatabase(tasks.PostgreSQLCreateDatabaseConfig{
			DatabaseName: database.Name,
			Owner:        "postgres",
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
		return fmt.Errorf("failed to create database: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to create database: %s", result.GetOutput())
	}

	// Mark the database as installed
	now := time.Now()
	if err := j.db.Model(&database).Update("installed_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update database status: %w", err)
	}

	j.broadcastProgress(database.ServerID, payload.DatabaseID, "installed", fmt.Sprintf("Database %s created successfully", database.Name))

	return nil
}

// Failed handles job failure
func (j *InstallDatabaseJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload InstallDatabasePayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("database_id", payload.DatabaseID).
		Msg("Failed to install database")

	// Fetch the database to get server ID for broadcasting
	var database models.Database
	if findErr := j.db.First(&database, "id = ?", payload.DatabaseID).Error; findErr != nil {
		return
	}

	// Mark installation as failed
	now := time.Now()
	j.db.Model(&database).Update("installation_failed_at", &now)

	j.broadcastProgress(database.ServerID, payload.DatabaseID, "failed", fmt.Sprintf("Failed to create database: %s", database.Name))
}

func (j *InstallDatabaseJob) broadcastProgress(serverID, databaseID, status, message string) {
	j.ws.BroadcastToServer(serverID, "database.progress", map[string]interface{}{
		"server_id":   serverID,
		"database_id": databaseID,
		"status":      status,
		"message":     message,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

// getDatabaseType determines the database type from server's installed services
func (j *InstallDatabaseJob) getDatabaseType(serverID string) string {
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
