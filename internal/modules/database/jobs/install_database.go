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

// InstallDatabaseJob handles database installation on a server.
// Similar to Laravel's Modules\Database\Jobs\InstallDatabase
type InstallDatabaseJob struct {
	DatabaseJobBase
	jobs.InstallationTracker
	Payload InstallDatabasePayload
}

// Type returns the job type identifier.
func (j *InstallDatabaseJob) Type() string {
	return TypeInstallDatabase
}

// Handle processes the install database job.
func (j *InstallDatabaseJob) Handle(ctx context.Context) error {
	j.LogInfo("Installing database", "database_id", j.Payload.DatabaseID)

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

	j.broadcastProgress(database.ServerID, j.Payload.DatabaseID, "installing", fmt.Sprintf("Creating database: %s", database.Name))

	// Determine database type from installed services
	dbType := j.getDatabaseType(ctx, database.ServerID)

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
	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to create database: %s", result.GetOutput())
	}

	// Mark the database as installed
	if err := j.MarkAsInstalled(j.DB, database); err != nil {
		return fmt.Errorf("failed to update database status: %w", err)
	}

	j.broadcastProgress(database.ServerID, j.Payload.DatabaseID, "installed", fmt.Sprintf("Database %s created successfully", database.Name))

	return nil
}

// Failed is called when the job fails after all retries.
func (j *InstallDatabaseJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to install database", "database_id", j.Payload.DatabaseID)

	// Fetch the database to get server ID for broadcasting
	database, findErr := repository.Find[models.Database](j.DB, ctx, j.Payload.DatabaseID)
	if findErr != nil {
		return
	}

	// Mark installation as failed
	j.MarkInstallationFailed(j.DB, database)

	j.broadcastProgress(database.ServerID, j.Payload.DatabaseID, "failed", fmt.Sprintf("Failed to create database: %s", database.Name))
}

func (j *InstallDatabaseJob) broadcastProgress(serverID, databaseID, status, message string) {
	j.BroadcastDatabaseEvent(serverID, "database.progress", map[string]any{
		"server_id":   serverID,
		"database_id": databaseID,
		"status":      status,
		"message":     message,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

// getDatabaseType determines the database type from server's installed services.
func (j *InstallDatabaseJob) getDatabaseType(ctx context.Context, serverID string) string {
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
