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

const TypeUpdateDatabaseUser = "database:user:update"

// UpdateDatabaseUserPayload contains data for updating a database user
type UpdateDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	Password       *string `json:"password,omitempty"`
	CallerID       *string `json:"caller_id,omitempty"`
}

// UpdateDatabaseUserJob handles database user updates on a server
type UpdateDatabaseUserJob struct {
	db         *gorm.DB
	ws         *websocket.Hub
	dispatcher *taskrunner.Dispatcher
	logger     *zerolog.Logger
}

// NewUpdateDatabaseUserJob creates a new update database user job handler
func NewUpdateDatabaseUserJob(db *gorm.DB, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger) *UpdateDatabaseUserJob {
	return &UpdateDatabaseUserJob{
		db:         db,
		ws:         ws,
		dispatcher: dispatcher,
		logger:     logger,
	}
}

// NewUpdateDatabaseUserTask creates a new asynq task for updating a database user
func NewUpdateDatabaseUserTask(databaseUserID string, password *string, callerID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(UpdateDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeUpdateDatabaseUser, payload), nil
}

// Handle processes the update database user job
func (j *UpdateDatabaseUserJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload UpdateDatabaseUserPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Updating database user")

	// Fetch the database user with databases
	var dbUser models.DatabaseUser
	if err := j.db.Preload("Databases").First(&dbUser, "id = ?", payload.DatabaseUserID).Error; err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	// Fetch the server
	var server servermodels.Server
	if err := j.db.First(&server, "id = ?", dbUser.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "updating", fmt.Sprintf("Updating database user: %s", dbUser.Name))

	// Update password if provided
	if payload.Password != nil && *payload.Password != "" {
		// Determine database type from installed services
		dbType := j.getDatabaseType(dbUser.ServerID)

		var task taskrunner.Task
		if dbType == "mysql" {
			task = tasks.MySQLUpdatePassword(tasks.MySQLUpdatePasswordConfig{
				AdminUser:     "root",
				AdminPassword: server.DatabasePassword.String(),
				Username:      dbUser.Name,
				NewPassword:   *payload.Password,
				Hosts:         []string{"%"},
			})
		} else {
			task = tasks.PostgreSQLUpdatePassword(tasks.PostgreSQLUpdatePasswordConfig{
				Username:    dbUser.Name,
				NewPassword: *payload.Password,
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
			return fmt.Errorf("failed to update database user password: %w", err)
		}

		if !result.IsSuccessful() {
			return fmt.Errorf("failed to update database user password: %s", result.GetOutput())
		}
	}

	// Update the database user record
	now := time.Now()
	if err := j.db.Model(&dbUser).Update("updated_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update database user record: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "updated", fmt.Sprintf("Database user %s updated successfully", dbUser.Name))

	return nil
}

// Failed handles job failure
func (j *UpdateDatabaseUserJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload UpdateDatabaseUserPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Failed to update database user")

	// Fetch the database user to get server ID for broadcasting
	var dbUser models.DatabaseUser
	if findErr := j.db.First(&dbUser, "id = ?", payload.DatabaseUserID).Error; findErr != nil {
		return
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to update database user: %s", dbUser.Name))
}

func (j *UpdateDatabaseUserJob) broadcastProgress(serverID, userID, status, message string) {
	j.ws.BroadcastToServer(serverID, "database_user.progress", map[string]interface{}{
		"server_id": serverID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// getDatabaseType determines the database type from server's installed services
func (j *UpdateDatabaseUserJob) getDatabaseType(serverID string) string {
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
