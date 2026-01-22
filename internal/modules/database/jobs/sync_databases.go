package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	dbmodels "github.com/kkz6/launch-go/internal/modules/database/models"
	dbtasks "github.com/kkz6/launch-go/internal/modules/database/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeSyncDatabases = "database:sync"

// SyncDatabasesPayload holds data for database sync job
type SyncDatabasesPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

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

type SyncDatabasesJob struct {
	Deps    *JobDeps
	Payload SyncDatabasesPayload

	server *servermodels.Server
}

func NewSyncDatabasesJob(p SyncDatabasesPayload) pkgjobs.Handler {
	return &SyncDatabasesJob{Deps: deps, Payload: p}
}

func (j *SyncDatabasesJob) Handle(ctx context.Context) error {
	j.Deps.Logger.Info().
		Str("server_id", j.Payload.ServerID).
		Msg("Syncing databases from server")

	var err error
	j.server, err = j.Deps.GetServerWithServices(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.BroadcastDatabaseProgress(j.server, "database.sync.progress", "", "syncing", "Syncing databases from server...")

	dbServiceType := j.getDatabaseServiceType(j.server)
	if dbServiceType == "" {
		j.Deps.Logger.Info().
			Str("server_id", j.Payload.ServerID).
			Msg("No database service found on server, skipping sync")
		j.Deps.BroadcastDatabaseProgress(j.server, "database.sync.progress", "", "synced", "No database service found on server")
		return nil
	}

	serverDatabases, err := j.getDatabasesFromServer(ctx, j.server, dbServiceType)
	if err != nil {
		return fmt.Errorf("failed to get databases from server: %w", err)
	}

	var protectedDatabases []string
	if dbServiceType == types.ServiceTypeMySQL {
		protectedDatabases = protectedMySQLDatabases
	} else {
		protectedDatabases = protectedPostgreSQLDatabases
	}

	userDatabases := filterProtectedDatabases(serverDatabases, protectedDatabases)

	j.Deps.Logger.Info().
		Str("server_id", j.Payload.ServerID).
		Int("total_databases", len(serverDatabases)).
		Int("user_databases", len(userDatabases)).
		Msg("Found databases on server")

	existingDatabases, err := j.Deps.Repos.Database().FindByServer(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to get existing databases: %w", err)
	}

	existingNames := make(map[string]bool)
	for _, db := range existingDatabases {
		existingNames[db.Name] = true
	}

	var syncedCount int
	for _, dbName := range userDatabases {
		if existingNames[dbName] {
			continue
		}

		database := &dbmodels.Database{
			Name: dbName,
		}
		database.ServerID = j.Payload.ServerID
		// Set as installed before creation (synced databases are already installed on server)
		now := time.Now()
		database.InstalledAt = &now
		database.InstallationFailedAt = nil

		if err := j.Deps.DB.WithContext(ctx).Create(database).Error; err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("server_id", j.Payload.ServerID).
				Str("database_name", dbName).
				Msg("Failed to create database record")
			continue
		}

		syncedCount++
		j.Deps.Logger.Info().
			Str("server_id", j.Payload.ServerID).
			Str("database_name", dbName).
			Msg("Synced database from server")
	}

	j.Deps.Logger.Info().
		Str("server_id", j.Payload.ServerID).
		Int("total_server_databases", len(userDatabases)).
		Int("existing_in_app", len(existingDatabases)).
		Int("synced_databases", syncedCount).
		Msg("Database sync completed")

	j.Deps.BroadcastDatabaseProgress(j.server, "database.sync.progress", "", "synced", fmt.Sprintf("Database sync completed. Found %d databases, synced %d new.", len(userDatabases), syncedCount))

	return nil
}

func (j *SyncDatabasesJob) getDatabaseServiceType(server *servermodels.Server) types.ServiceType {
	for _, service := range server.Services {
		if service.Type.IsDatabase() && service.Status.IsActive() {
			return service.Type
		}
	}
	return ""
}

func (j *SyncDatabasesJob) getDatabasesFromServer(ctx context.Context, server *servermodels.Server, dbType types.ServiceType) ([]string, error) {
	factory := dbtasks.NewFactory(dbType)
	task := factory.GetDatabases(dbtasks.GetDatabasesConfig{
		AdminUser:     "root",
		AdminPassword: server.DatabasePassword.String(),
	})

	result, err := j.Deps.RunTask(server, task).AsRoot().Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run get databases task: %w", err)
	}

	if !result.IsSuccessful() {
		return nil, fmt.Errorf("get databases command failed with exit code %d: %s", result.GetExitCode(), result.GetOutput())
	}

	databases := parseLines(result.GetOutput())

	return databases, nil
}

func (j *SyncDatabasesJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("Failed to sync databases")

	if j.server == nil {
		server, findErr := j.Deps.GetServer(ctx, j.Payload.ServerID)
		if findErr != nil {
			return
		}
		j.server = server
	}

	j.Deps.BroadcastDatabaseProgress(j.server, "database.sync.progress", "", "failed", "Failed to sync databases from server")
}

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

// NewSyncDatabasesTask creates a database sync job
// Uses TaskID for deduplication to prevent duplicate sync operations
func NewSyncDatabasesTask(serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeSyncDatabases, SyncDatabasesPayload{
		ServerID: serverID,
		UserID:   userID,
	}, asynq.TaskID(pkgjobs.Dedup("sync_databases", serverID)))
}
