package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	dbmodels "github.com/kkz6/launch-go/internal/modules/database/models"
	dbtasks "github.com/kkz6/launch-go/internal/modules/database/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Protected databases that should not be synced
var protectedMySQLDatabases = []string{
	"information_schema",
	"mysql",
	"performance_schema",
	"sys",
}

var protectedPostgreSQLDatabases = []string{
	"postgres",
	"template0",
	"template1",
}

// SyncDatabasesJob handles syncing databases from a server.
// Similar to Laravel's Modules\Database\Jobs\SyncDatabases
type SyncDatabasesJob struct {
	DatabaseJobBase
	Payload SyncDatabasesPayload
}

// Type returns the job type identifier.
func (j *SyncDatabasesJob) Type() string {
	return TypeSyncDatabases
}

// Handle processes the sync databases job.
func (j *SyncDatabasesJob) Handle(ctx context.Context) error {
	j.LogInfo("Syncing databases from server", "server_id", j.Payload.ServerID)

	j.broadcastProgress(j.Payload.ServerID, "syncing", "Syncing databases from server...")

	// Fetch the server with its services
	server, err := repository.NewQuery[servermodels.Server](j.DB, ctx).
		WithModel("Server").
		Preload("Services").
		FindByID(j.Payload.ServerID).
		FirstOrFail()
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Find the database service type
	dbServiceType := j.getDatabaseServiceType(server)
	if dbServiceType == "" {
		j.LogInfo("No database service found on server, skipping sync", "server_id", j.Payload.ServerID)
		j.broadcastProgress(j.Payload.ServerID, "synced", "No database service found on server")
		return nil
	}

	// Get databases from the server
	serverDatabases, err := j.getDatabasesFromServer(ctx, server, dbServiceType)
	if err != nil {
		return fmt.Errorf("failed to get databases from server: %w", err)
	}

	// Filter out protected/system databases
	var protectedDatabases []string
	if dbServiceType == enums.ServiceTypeMySql {
		protectedDatabases = protectedMySQLDatabases
	} else {
		protectedDatabases = protectedPostgreSQLDatabases
	}

	userDatabases := filterProtectedDatabases(serverDatabases, protectedDatabases)

	j.LogInfo("Found databases on server",
		"server_id", j.Payload.ServerID,
		"total_databases", len(serverDatabases),
		"user_databases", len(userDatabases),
	)

	// Get existing databases in the application
	existingDatabases, err := repository.NewQuery[dbmodels.Database](j.DB, ctx).
		Where("server_id = ?", j.Payload.ServerID).
		All()
	if err != nil {
		return fmt.Errorf("failed to get existing databases: %w", err)
	}

	existingNames := make(map[string]bool)
	for _, db := range existingDatabases {
		existingNames[db.Name] = true
	}

	// Find databases that exist on server but not in application
	var syncedCount int
	for _, dbName := range userDatabases {
		if existingNames[dbName] {
			continue
		}

		// Create missing database record
		database := &dbmodels.Database{
			ServerID: j.Payload.ServerID,
			Name:     dbName,
		}
		database.MarkAsInstalled() // Mark as installed since it exists on server

		if err := j.DB.WithContext(ctx).Create(database).Error; err != nil {
			j.LogError(err, "Failed to create database record",
				"server_id", j.Payload.ServerID,
				"database_name", dbName,
			)
			continue
		}

		syncedCount++
		j.LogInfo("Synced database from server",
			"server_id", j.Payload.ServerID,
			"database_name", dbName,
		)
	}

	j.LogInfo("Database sync completed",
		"server_id", j.Payload.ServerID,
		"total_server_databases", len(userDatabases),
		"existing_in_app", len(existingDatabases),
		"synced_databases", syncedCount,
	)

	j.broadcastProgress(j.Payload.ServerID, "synced", fmt.Sprintf("Database sync completed. Found %d databases, synced %d new.", len(userDatabases), syncedCount))

	return nil
}

// getDatabaseServiceType finds the database service type on the server.
func (j *SyncDatabasesJob) getDatabaseServiceType(server *servermodels.Server) enums.ServiceType {
	for _, service := range server.Services {
		if service.Type.IsDatabase() && service.Status.IsActive() {
			return service.Type
		}
	}
	return ""
}

// getDatabasesFromServer retrieves the list of databases from the server via SSH.
func (j *SyncDatabasesJob) getDatabasesFromServer(ctx context.Context, server *servermodels.Server, dbType enums.ServiceType) ([]string, error) {
	var task taskrunner.Task

	switch dbType {
	case enums.ServiceTypeMySql:
		task = dbtasks.MySQLGetDatabases(dbtasks.MySQLGetDatabasesConfig{})
	case enums.ServiceTypePostgreSql:
		task = dbtasks.PostgreSQLGetDatabases()
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	// Create pending task with SSH connection
	pt, err := j.createPendingTask(server, task)
	if err != nil {
		return nil, fmt.Errorf("failed to create pending task: %w", err)
	}

	// Run the task
	result, err := j.Dispatcher.Run(ctx, pt)
	if err != nil {
		return nil, fmt.Errorf("failed to run get databases task: %w", err)
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("get databases command failed with exit code %d: %s", result.ExitCode, result.Output)
	}

	// Parse the output - each line is a database name
	databases := parseLines(result.Output)

	return databases, nil
}

// createPendingTask creates a pending task with SSH connection for the server.
func (j *SyncDatabasesJob) createPendingTask(server *servermodels.Server, task taskrunner.Task) (*taskrunner.PendingTask, error) {
	// Get SSH credentials
	if server.PrivateKey.IsEmpty() {
		return nil, fmt.Errorf("server has no private key configured")
	}

	publicIP := ""
	if server.PublicIPv4 != nil {
		publicIP = *server.PublicIPv4
	}

	if publicIP == "" {
		return nil, fmt.Errorf("server has no public IP configured")
	}

	return &taskrunner.PendingTask{
		Task: task,
		Connection: &taskrunner.Connection{
			Host:       publicIP,
			Port:       server.GetSSHPort(),
			User:       server.GetUsername(),
			PrivateKey: server.PrivateKey.String(),
		},
	}, nil
}

// Failed is called when the job fails after all retries.
func (j *SyncDatabasesJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to sync databases", "server_id", j.Payload.ServerID)

	j.broadcastProgress(j.Payload.ServerID, "failed", "Failed to sync databases from server")
}

func (j *SyncDatabasesJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastDatabaseEvent(serverID, "database.sync.progress", map[string]any{
		"server_id": serverID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// filterProtectedDatabases removes protected databases from the list.
func filterProtectedDatabases(databases, protected []string) []string {
	protectedMap := make(map[string]bool)
	for _, p := range protected {
		protectedMap[strings.ToLower(p)] = true
	}

	var filtered []string
	for _, db := range databases {
		if !protectedMap[strings.ToLower(db)] {
			filtered = append(filtered, db)
		}
	}

	return filtered
}

// parseLines splits the output into lines and filters empty ones.
func parseLines(output string) []string {
	lines := strings.Split(output, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
