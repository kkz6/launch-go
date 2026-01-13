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

// UninstallDatabaseJob handles database uninstallation from a server.
// Similar to Laravel's Modules\Database\Jobs\UninstallDatabase
type UninstallDatabaseJob struct {
	DatabaseJobBase
	jobs.UninstallationTracker
	Payload UninstallDatabasePayload
}

// Type returns the job type identifier.
func (j *UninstallDatabaseJob) Type() string {
	return TypeUninstallDatabase
}

// Handle processes the uninstall database job.
func (j *UninstallDatabaseJob) Handle(ctx context.Context) error {
	j.LogInfo("Uninstalling database", "database_id", j.Payload.DatabaseID)

	// Fetch the database
	database, err := repository.Find[models.Database](j.DB, ctx, j.Payload.DatabaseID)
	if err != nil {
		return fmt.Errorf("failed to find database: %w", err)
	}

	// Fetch the server
	server, err := repository.Find[servermodels.Server](j.DB, ctx, database.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.broadcastProgress(database.ServerID, j.Payload.DatabaseID, "uninstalling", fmt.Sprintf("Dropping database: %s", database.Name))

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
	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	if !result.IsSuccessful() {
		j.LogInfo("Database drop completed with errors", "output", result.GetOutput())
	}

	// Delete the database record
	if err := j.MarkAsUninstalled(j.DB, database); err != nil {
		return fmt.Errorf("failed to delete database record: %w", err)
	}

	j.broadcastProgress(database.ServerID, j.Payload.DatabaseID, "deleted", fmt.Sprintf("Database %s deleted successfully", database.Name))

	return nil
}

// Failed is called when the job fails after all retries.
func (j *UninstallDatabaseJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to uninstall database", "database_id", j.Payload.DatabaseID)

	// Fetch the database to get server ID for broadcasting
	database, findErr := repository.Find[models.Database](j.DB, ctx, j.Payload.DatabaseID)
	if findErr != nil {
		return
	}

	j.broadcastProgress(database.ServerID, j.Payload.DatabaseID, "failed", fmt.Sprintf("Failed to delete database: %s", database.Name))
}

func (j *UninstallDatabaseJob) broadcastProgress(serverID, databaseID, status, message string) {
	j.BroadcastDatabaseEvent(serverID, "database.progress", map[string]any{
		"server_id":   serverID,
		"database_id": databaseID,
		"status":      status,
		"message":     message,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

// getDatabaseType determines the database type from server's installed services.
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
