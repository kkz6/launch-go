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
	return jobs.NewTask(TypeUpdateDatabaseUser, UpdateDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
	})
}

// Handle processes the update database user job
func (j *UpdateDatabaseUserJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.ParsePayload[UpdateDatabaseUserPayload](t)
	if err != nil {
		return err
	}

	j.logger.Info().
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Updating database user")

	// Fetch the database user with databases using OrFail pattern
	dbUser, err := repository.NewQuery[models.DatabaseUser](j.db, ctx).
		WithModel("DatabaseUser").
		Preload("Databases").
		FindByID(payload.DatabaseUserID).
		FirstOrFail()
	if err != nil {
		return err
	}

	// Fetch the server using OrFail pattern
	server, err := repository.Find[servermodels.Server](j.db, ctx, dbUser.ServerID)
	if err != nil {
		return err
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "updating", fmt.Sprintf("Updating database user: %s", dbUser.Name))

	// Update password if provided
	if payload.Password != nil && *payload.Password != "" {
		// Determine database type from installed services
		dbType := j.getDatabaseType(ctx, dbUser.ServerID)

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
		result, err := servertasks.NewTaskRunner(server, task).
			WithDB(j.db).
			WithDispatcher(j.dispatcher).
			WithLogger(j.logger).
			AsRoot().
			Run(ctx)
		if err != nil {
			return fmt.Errorf("failed to update database user password: %w", err)
		}

		if !result.IsSuccessful() {
			return fmt.Errorf("failed to update database user password: %s", result.GetOutput())
		}
	}

	// Update the database user record
	now := time.Now()
	if err := j.db.WithContext(ctx).Model(dbUser).Update("updated_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update database user record: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, payload.DatabaseUserID, "updated", fmt.Sprintf("Database user %s updated successfully", dbUser.Name))

	return nil
}

// Failed handles job failure
func (j *UpdateDatabaseUserJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, parseErr := jobs.ParsePayload[UpdateDatabaseUserPayload](t)
	if parseErr != nil {
		j.logger.Error().Err(parseErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("database_user_id", payload.DatabaseUserID).
		Msg("Failed to update database user")

	// Fetch the database user to get server ID for broadcasting
	dbUser, findErr := repository.Find[models.DatabaseUser](j.db, ctx, payload.DatabaseUserID)
	if findErr != nil {
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
func (j *UpdateDatabaseUserJob) getDatabaseType(ctx context.Context, serverID string) string {
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
