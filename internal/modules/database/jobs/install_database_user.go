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

const TypeInstallDatabaseUser = "database:user:install"

// InstallDatabaseUserPayload contains data for installing a database user
type InstallDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	Password       string  `json:"password"`
	CallerID       *string `json:"caller_id,omitempty"`
}

// InstallDatabaseUserJob handles database user installation on a server
type InstallDatabaseUserJob struct {
	db         *gorm.DB
	ws         *websocket.Hub
	dispatcher *taskrunner.Dispatcher
	logger     *zerolog.Logger
}

// NewInstallDatabaseUserJob creates a new install database user job handler
func NewInstallDatabaseUserJob(db *gorm.DB, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger) *InstallDatabaseUserJob {
	return &InstallDatabaseUserJob{
		db:         db,
		ws:         ws,
		dispatcher: dispatcher,
		logger:     logger,
	}
}

// NewInstallDatabaseUserTask creates a new asynq task for installing a database user
func NewInstallDatabaseUserTask(databaseUserID, password string, callerID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeInstallDatabaseUser, payload), nil
}

// Handle processes the install database user job
func (j *InstallDatabaseUserJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload InstallDatabaseUserPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Installing database user")

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

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "installing", fmt.Sprintf("Creating database user: %s", dbUser.Name))

	// Determine database type from installed services
	dbType := j.getDatabaseType(dbUser.ServerID)

	// Create TaskRunner deps
	taskRunnerDeps := &servertasks.TaskRunnerDeps{
		DB:         j.db,
		Dispatcher: j.dispatcher,
		Logger:     j.logger,
	}

	if dbType == "mysql" {
		// Create MySQL user
		createUserTask := tasks.MySQLCreateUser(tasks.MySQLCreateUserConfig{
			AdminUser:     "root",
			AdminPassword: server.DatabasePassword.String(),
			Username:      dbUser.Name,
			UserPassword:  payload.Password,
			Hosts:         []string{"%"},
		})

		result, err := taskRunnerDeps.NewRunner(&server, createUserTask).AsRoot().Run(ctx)
		if err != nil {
			return fmt.Errorf("failed to create database user: %w", err)
		}

		if !result.IsSuccessful() {
			return fmt.Errorf("failed to create database user: %s", result.GetOutput())
		}

		// Grant privileges on each associated database
		for _, db := range dbUser.Databases {
			grantTask := tasks.MySQLGrantPrivileges(tasks.MySQLGrantPrivilegesConfig{
				AdminUser:     "root",
				AdminPassword: server.DatabasePassword.String(),
				Username:      dbUser.Name,
				DatabaseName:  db.Name,
				Hosts:         []string{"%"},
			})

			result, err := taskRunnerDeps.NewRunner(&server, grantTask).AsRoot().Run(ctx)
			if err != nil {
				j.logger.Warn().Err(err).Str("database", db.Name).Msg("Failed to grant privileges")
			} else if !result.IsSuccessful() {
				j.logger.Warn().Str("output", result.GetOutput()).Str("database", db.Name).Msg("Grant privileges completed with errors")
			}
		}
	} else {
		// Create PostgreSQL user
		createUserTask := tasks.PostgreSQLCreateUser(tasks.PostgreSQLCreateUserConfig{
			Username: dbUser.Name,
			Password: payload.Password,
		})

		result, err := taskRunnerDeps.NewRunner(&server, createUserTask).AsRoot().Run(ctx)
		if err != nil {
			return fmt.Errorf("failed to create database user: %w", err)
		}

		if !result.IsSuccessful() {
			return fmt.Errorf("failed to create database user: %s", result.GetOutput())
		}

		// Grant privileges on each associated database
		for _, db := range dbUser.Databases {
			grantTask := tasks.PostgreSQLGrantPrivileges(tasks.PostgreSQLGrantPrivilegesConfig{
				Username:     dbUser.Name,
				DatabaseName: db.Name,
			})

			result, err := taskRunnerDeps.NewRunner(&server, grantTask).AsRoot().Run(ctx)
			if err != nil {
				j.logger.Warn().Err(err).Str("database", db.Name).Msg("Failed to grant privileges")
			} else if !result.IsSuccessful() {
				j.logger.Warn().Str("output", result.GetOutput()).Str("database", db.Name).Msg("Grant privileges completed with errors")
			}
		}
	}

	// Mark the database user as installed
	now := time.Now()
	if err := j.db.Model(&dbUser).Update("installed_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update database user status: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "installed", fmt.Sprintf("Database user %s created successfully", dbUser.Name))

	return nil
}

// Failed handles job failure
func (j *InstallDatabaseUserJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload InstallDatabaseUserPayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Failed to install database user")

	// Fetch the database user to get server ID for broadcasting
	var dbUser models.DatabaseUser
	if findErr := j.db.First(&dbUser, "id = ?", payload.DatabaseUserID).Error; findErr != nil {
		return
	}

	// Mark installation as failed
	now := time.Now()
	j.db.Model(&dbUser).Update("installation_failed_at", &now)

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to create database user: %s", dbUser.Name))
}

func (j *InstallDatabaseUserJob) broadcastProgress(serverID, userID, status, message string) {
	j.ws.BroadcastToServer(serverID, "database_user.progress", map[string]interface{}{
		"server_id": serverID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// getDatabaseType determines the database type from server's installed services
func (j *InstallDatabaseUserJob) getDatabaseType(serverID string) string {
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
