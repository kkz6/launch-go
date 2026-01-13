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
	"github.com/kkz6/launch-go/internal/websocket"
)

const TypeUninstallDatabaseUser = "database:user:uninstall"

// UninstallDatabaseUserPayload contains data for uninstalling a database user
type UninstallDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	CallerID       *string `json:"caller_id,omitempty"`
}

// UninstallDatabaseUserJob handles database user uninstallation from a server
type UninstallDatabaseUserJob struct {
	db         *gorm.DB
	ws         *websocket.Hub
	dispatcher *taskrunner.Dispatcher
	logger     *zerolog.Logger
}

// NewUninstallDatabaseUserJob creates a new uninstall database user job handler
func NewUninstallDatabaseUserJob(db *gorm.DB, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger) *UninstallDatabaseUserJob {
	return &UninstallDatabaseUserJob{
		db:         db,
		ws:         ws,
		dispatcher: dispatcher,
		logger:     logger,
	}
}

// NewUninstallDatabaseUserTask creates a new asynq task for uninstalling a database user
func NewUninstallDatabaseUserTask(databaseUserID string, callerID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallDatabaseUser, UninstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		CallerID:       callerID,
	})
}

// Handle processes the uninstall database user job
func (j *UninstallDatabaseUserJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.ParsePayload[UninstallDatabaseUserPayload](t)
	if err != nil {
		return err
	}

	j.logger.Info().
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Uninstalling database user")

	// Fetch the database user
	dbUser, err := repository.Find[models.DatabaseUser](j.db, ctx, payload.DatabaseUserID)
	if err != nil {
		return err
	}

	// Fetch the server
	server, err := repository.Find[servermodels.Server](j.db, ctx, dbUser.ServerID)
	if err != nil {
		return err
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "uninstalling", fmt.Sprintf("Dropping database user: %s", dbUser.Name))

	// Determine database type from installed services
	dbType := j.getDatabaseType(ctx, dbUser.ServerID)

	// Create drop user task
	var task taskrunner.Task
	if dbType == "mysql" {
		task = tasks.MySQLDropUser(tasks.MySQLDropUserConfig{
			AdminUser:     "root",
			AdminPassword: server.DatabasePassword.String(),
			Username:      dbUser.Name,
			Hosts:         []string{"%"},
		})
	} else {
		task = tasks.PostgreSQLDropUser(tasks.PostgreSQLDropUserConfig{
			Username: dbUser.Name,
		})
	}

	// Use TaskRunner to execute on server
	result, err := servertasks.NewTaskRunner(server, task).
		WithDB(j.db).
		WithDispatcher(j.dispatcher).
		WithLogger(j.logger).
		AsRoot().
		Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop database user: %w", err)
	}

	if !result.IsSuccessful() {
		j.logger.Warn().
			Str("output", result.GetOutput()).
			Msg("Database user drop completed with errors")
	}

	// Delete the database user record
	if err := j.db.WithContext(ctx).Delete(dbUser).Error; err != nil {
		return fmt.Errorf("failed to delete database user record: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "deleted", fmt.Sprintf("Database user %s deleted successfully", dbUser.Name))

	return nil
}

// Failed handles job failure
func (j *UninstallDatabaseUserJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, parseErr := jobs.ParsePayload[UninstallDatabaseUserPayload](t)
	if parseErr != nil {
		j.logger.Error().Err(parseErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Failed to uninstall database user")

	// Fetch the database user to get server ID for broadcasting
	dbUser, findErr := repository.Find[models.DatabaseUser](j.db, ctx, payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to delete database user: %s", dbUser.Name))
}

func (j *UninstallDatabaseUserJob) broadcastProgress(serverID, userID, status, message string) {
	j.ws.BroadcastToServer(serverID, "database_user.progress", map[string]interface{}{
		"server_id": serverID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// getDatabaseType determines the database type from server's installed services
func (j *UninstallDatabaseUserJob) getDatabaseType(ctx context.Context, serverID string) string {
	service, err := repository.NewQuery[servermodels.InstalledService](j.db, ctx).
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
