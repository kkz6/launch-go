package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	serverenums "github.com/kkz6/launch-go/internal/modules/server/enums"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// UninstallDatabaseUserJob handles database user uninstallation from a server.
// Similar to Laravel's Modules\Database\Jobs\UninstallDatabaseUser
type UninstallDatabaseUserJob struct {
	DatabaseJobBase
	jobs.UninstallationTracker
	Payload UninstallDatabaseUserPayload
}

// Type returns the job type identifier.
func (j *UninstallDatabaseUserJob) Type() string {
	return TypeUninstallDatabaseUser
}

// Handle processes the uninstall database user job.
func (j *UninstallDatabaseUserJob) Handle(ctx context.Context) error {
	j.LogInfo("Uninstalling database user", "database_user_id", j.Payload.DatabaseUserID)

	// Fetch the database user
	dbUser, err := repository.Find[models.DatabaseUser](j.DB, ctx, j.Payload.DatabaseUserID)
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	// Fetch the server
	server, err := repository.Find[servermodels.Server](j.DB, ctx, dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, j.Payload.DatabaseUserID, "uninstalling", fmt.Sprintf("Dropping database user: %s", dbUser.Name))

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
	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop database user: %w", err)
	}

	if !result.IsSuccessful() {
		j.LogInfo("Database user drop completed with errors", "output", result.GetOutput())
	}

	// Delete the database user record
	if err := j.MarkAsUninstalled(j.DB, dbUser); err != nil {
		return fmt.Errorf("failed to delete database user record: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, j.Payload.DatabaseUserID, "deleted", fmt.Sprintf("Database user %s deleted successfully", dbUser.Name))

	return nil
}

// Failed is called when the job fails after all retries.
func (j *UninstallDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to uninstall database user", "database_user_id", j.Payload.DatabaseUserID)

	// Fetch the database user to get server ID for broadcasting
	dbUser, findErr := repository.Find[models.DatabaseUser](j.DB, ctx, j.Payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	j.broadcastProgress(dbUser.ServerID, j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to delete database user: %s", dbUser.Name))
}

func (j *UninstallDatabaseUserJob) broadcastProgress(serverID, userID, status, message string) {
	j.BroadcastDatabaseEvent(serverID, "database_user.progress", map[string]any{
		"server_id": serverID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// getDatabaseType determines the database type from server's installed services.
func (j *UninstallDatabaseUserJob) getDatabaseType(ctx context.Context, serverID string) string {
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
