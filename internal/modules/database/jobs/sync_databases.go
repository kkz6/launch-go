package jobs

import (
	"context"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"

	dbmodels "github.com/kkz6/launch-go/internal/modules/database/models"
	dbtasks "github.com/kkz6/launch-go/internal/modules/database/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/repository"
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
	pkgjobs.BaseJob[*JobContext, SyncDatabasesPayload]
}

func NewSyncDatabasesJob(ctx *JobContext, payload SyncDatabasesPayload) *SyncDatabasesJob {
	return &SyncDatabasesJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

func (j *SyncDatabasesJob) Handle(ctx context.Context) error {
	j.Ctx.LogInfo("Syncing databases from server", "server_id", j.Payload.ServerID)

	server, err := repository.NewQuery[servermodels.Server](ctx, j.Ctx.DB).
		WithModel("Server").
		Preload("Services").
		FindByID(j.Payload.ServerID).
		FirstOrFail()
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.BroadcastDatabaseProgress(server, "database.sync.progress", "", "syncing", "Syncing databases from server...")

	dbServiceType := j.getDatabaseServiceType(server)
	if dbServiceType == "" {
		j.Ctx.LogInfo("No database service found on server, skipping sync", "server_id", j.Payload.ServerID)
		j.Ctx.BroadcastDatabaseProgress(server, "database.sync.progress", "", "synced", "No database service found on server")
		return nil
	}

	serverDatabases, err := j.getDatabasesFromServer(ctx, server, dbServiceType)
	if err != nil {
		return fmt.Errorf("failed to get databases from server: %w", err)
	}

	var protectedDatabases []string
	if dbServiceType == enums.ServiceTypeMySql {
		protectedDatabases = protectedMySQLDatabases
	} else {
		protectedDatabases = protectedPostgreSQLDatabases
	}

	userDatabases := filterProtectedDatabases(serverDatabases, protectedDatabases)

	j.Ctx.LogInfo("Found databases on server",
		"server_id", j.Payload.ServerID,
		"total_databases", len(serverDatabases),
		"user_databases", len(userDatabases),
	)

	existingDatabases, err := repository.NewQuery[dbmodels.Database](ctx, j.Ctx.DB).
		Where("server_id = ?", j.Payload.ServerID).
		All()
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
			ServerID: j.Payload.ServerID,
			Name:     dbName,
		}
		database.MarkAsInstalled()

		if err := j.Ctx.DB.WithContext(ctx).Create(database).Error; err != nil {
			j.Ctx.LogError(err, "Failed to create database record",
				"server_id", j.Payload.ServerID,
				"database_name", dbName,
			)
			continue
		}

		syncedCount++
		j.Ctx.LogInfo("Synced database from server",
			"server_id", j.Payload.ServerID,
			"database_name", dbName,
		)
	}

	j.Ctx.LogInfo("Database sync completed",
		"server_id", j.Payload.ServerID,
		"total_server_databases", len(userDatabases),
		"existing_in_app", len(existingDatabases),
		"synced_databases", syncedCount,
	)

	j.Ctx.BroadcastDatabaseProgress(server, "database.sync.progress", "", "synced", fmt.Sprintf("Database sync completed. Found %d databases, synced %d new.", len(userDatabases), syncedCount))

	return nil
}

func (j *SyncDatabasesJob) getDatabaseServiceType(server *servermodels.Server) enums.ServiceType {
	for _, service := range server.Services {
		if service.Type.IsDatabase() && service.Status.IsActive() {
			return service.Type
		}
	}
	return ""
}

func (j *SyncDatabasesJob) getDatabasesFromServer(ctx context.Context, server *servermodels.Server, dbType enums.ServiceType) ([]string, error) {
	factory := dbtasks.NewFactory(dbType)
	task := factory.GetDatabases(dbtasks.GetDatabasesConfig{
		AdminUser:     "root",
		AdminPassword: server.DatabasePassword.String(),
	})

	result, err := j.Ctx.RunTaskOnServer(server, task).AsRoot().Run(ctx)
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
	j.Ctx.LogError(err, "Failed to sync databases", "server_id", j.Payload.ServerID)

	server, findErr := repository.Find[servermodels.Server](ctx, j.Ctx.DB, j.Payload.ServerID)
	if findErr != nil {
		return
	}

	j.Ctx.BroadcastDatabaseProgress(server, "database.sync.progress", "", "failed", "Failed to sync databases from server")
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
	return pkgjobs.NewTask(TypeSyncDatabases, SyncDatabasesPayload{
		ServerID: serverID,
		UserID:   userID,
	}, asynq.TaskID(fmt.Sprintf("sync_databases:%s", serverID)))
}
