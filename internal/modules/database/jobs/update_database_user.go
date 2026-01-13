package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	serverenums "github.com/kkz6/launch-go/internal/modules/server/enums"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// UpdateDatabaseUserJob handles database user updates on a server.
// Similar to Laravel's Modules\Database\Jobs\UpdateDatabaseUser
type UpdateDatabaseUserJob struct {
	DatabaseJobBase
	Payload UpdateDatabaseUserPayload
}

// Type returns the job type identifier.
func (j *UpdateDatabaseUserJob) Type() string {
	return TypeUpdateDatabaseUser
}

// Handle processes the update database user job.
func (j *UpdateDatabaseUserJob) Handle(ctx context.Context) error {
	j.LogInfo("Updating database user", "database_user_id", j.Payload.DatabaseUserID)

	// Fetch the database user with databases
	dbUser, err := repository.NewQuery[models.DatabaseUser](j.DB, ctx).
		WithModel("DatabaseUser").
		Preload("Databases").
		FindByID(j.Payload.DatabaseUserID).
		FirstOrFail()
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	// Fetch the server
	server, err := repository.Find[servermodels.Server](j.DB, ctx, dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, j.Payload.DatabaseUserID, "updating", fmt.Sprintf("Updating database user: %s", dbUser.Name))

	// Update password if provided
	if j.Payload.Password != nil && *j.Payload.Password != "" {
		// Determine database type from installed services
		dbType := j.getDatabaseType(ctx, dbUser.ServerID)

		var task taskrunner.Task
		if dbType == "mysql" {
			task = tasks.MySQLUpdatePassword(tasks.MySQLUpdatePasswordConfig{
				AdminUser:     "root",
				AdminPassword: server.DatabasePassword.String(),
				Username:      dbUser.Name,
				NewPassword:   *j.Payload.Password,
				Hosts:         []string{"%"},
			})
		} else {
			task = tasks.PostgreSQLUpdatePassword(tasks.PostgreSQLUpdatePasswordConfig{
				Username:    dbUser.Name,
				NewPassword: *j.Payload.Password,
			})
		}

		// Use TaskRunner to execute on server
		result, err := j.RunTaskOnServer(server, task).
			AsRoot().
			Dispatch(ctx)
		if err != nil {
			return fmt.Errorf("failed to update database user password: %w", err)
		}

		if !result.IsSuccessful() {
			return fmt.Errorf("failed to update database user password: %s", result.GetOutput())
		}
	}

	// Update the database user record
	now := time.Now()
	if err := j.DB.WithContext(ctx).Model(dbUser).Update("updated_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update database user record: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, j.Payload.DatabaseUserID, "updated", fmt.Sprintf("Database user %s updated successfully", dbUser.Name))

	return nil
}

// Failed is called when the job fails after all retries.
func (j *UpdateDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to update database user", "database_user_id", j.Payload.DatabaseUserID)

	// Fetch the database user to get server ID for broadcasting
	dbUser, findErr := repository.Find[models.DatabaseUser](j.DB, ctx, j.Payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	j.broadcastProgress(dbUser.ServerID, j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to update database user: %s", dbUser.Name))
}

func (j *UpdateDatabaseUserJob) broadcastProgress(serverID, userID, status, message string) {
	j.BroadcastDatabaseEvent(serverID, "database_user.progress", map[string]any{
		"server_id": serverID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// getDatabaseType determines the database type from server's installed services.
func (j *UpdateDatabaseUserJob) getDatabaseType(ctx context.Context, serverID string) string {
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
