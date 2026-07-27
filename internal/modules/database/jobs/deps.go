package jobs

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/contracts"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	pkgtaskrunner "github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// JobDeps holds all dependencies for database jobs.
type JobDeps struct {
	*pkgjobs.Deps
	Repos          contracts.RepositoryRegistry
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

// NewJobDeps creates a new JobDeps from app dependencies.
func NewJobDeps(appDeps app.Deps, repos contracts.RepositoryRegistry) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          appDeps.DB,
			Logger:      appDeps.Logger,
			Queue:       appDeps.Queue,
			Broadcaster: appDeps.WebSocket,
			Dispatcher:  appDeps.Dispatcher,
		},
		Repos: repos,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:          appDeps.DB,
			Queue:       appDeps.Queue,
			Dispatcher:  appDeps.Dispatcher,
			Logger:      appDeps.Logger,
			Broadcaster: appDeps.WebSocket,
			Notifier:    appDeps.Notifier,
		},
	}
}

// NewJobDepsWithParams creates JobDeps with explicit parameters (for testing or custom setup).
func NewJobDepsWithParams(
	db *gorm.DB,
	repos contracts.RepositoryRegistry,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
	dispatcher pkgtaskrunner.TaskDispatcher,
	queueClient *queue.Client,
) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          db,
			Logger:      logger,
			Queue:       queueClient,
			Broadcaster: ws,
			Dispatcher:  dispatcher,
		},
		Repos: repos,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:          db,
			Queue:       queueClient,
			Dispatcher:  dispatcher,
			Logger:      logger,
			Broadcaster: ws,
		},
	}
}

// RunTask creates a task runner for a server with a task.
func (d *JobDeps) RunTask(server *servermodels.Server, task pkgtaskrunner.Task) *servertasks.TaskRunner {
	return d.TaskRunnerDeps.NewRunner(server, task).TrackInDB()
}

// BroadcastServerEvent broadcasts an event for a server to its team channel.
func (d *JobDeps) BroadcastServerEvent(server *servermodels.Server, event string, data any) {
	if server != nil {
		d.BroadcastToTeam(server.TeamID, event, data)
	}
}

// GetServer returns a server by ID.
func (d *JobDeps) GetServer(ctx context.Context, serverID string) (*servermodels.Server, error) {
	return repository.Find[servermodels.Server](ctx, d.DB, serverID)
}

// GetServerWithServices returns a server by ID with services preloaded.
func (d *JobDeps) GetServerWithServices(ctx context.Context, serverID string) (*servermodels.Server, error) {
	return repository.NewQuery[servermodels.Server](ctx, d.DB).
		Preload("Services").
		FindByID(serverID).
		FirstOrFail()
}

// GetDatabaseType returns the database type for a server (mysql or postgresql).
func (d *JobDeps) GetDatabaseType(ctx context.Context, serverID string) string {
	service := d.getDatabaseService(ctx, serverID)
	if service == nil {
		return "mysql"
	}

	if service.Type == servertypes.ServiceTypePostgreSQL {
		return "postgresql"
	}
	return "mysql"
}

// GetDatabaseServiceType returns the database service type for a server.
func (d *JobDeps) GetDatabaseServiceType(ctx context.Context, serverID string) servertypes.ServiceType {
	service := d.getDatabaseService(ctx, serverID)
	if service == nil {
		return servertypes.ServiceTypeMySQL
	}
	return service.Type
}

// getDatabaseService returns the database service for a server (internal helper).
func (d *JobDeps) getDatabaseService(ctx context.Context, serverID string) *servermodels.InstalledService {
	service, err := repository.NewQuery[servermodels.InstalledService](ctx, d.DB).
		Where("server_id = ? AND type IN ?", serverID, []string{
			string(servertypes.ServiceTypeMySQL),
			string(servertypes.ServiceTypePostgreSQL),
		}).
		First()

	if err != nil {
		return nil
	}
	return service
}

// GetTaskFactory returns a task factory for the server's database type.
func (d *JobDeps) GetTaskFactory(ctx context.Context, serverID string) *tasks.Factory {
	dbType := d.GetDatabaseServiceType(ctx, serverID)
	return tasks.NewFactory(dbType)
}

// BroadcastDatabaseProgress broadcasts a standard progress event for database operations.
func (d *JobDeps) BroadcastDatabaseProgress(server *servermodels.Server, event, entityID, status, message string) {
	data := map[string]any{
		"server_id": server.ID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	if entityID != "" {
		data["database_id"] = entityID
	}
	d.BroadcastServerEvent(server, event, data)
}

// BroadcastUserProgress broadcasts a standard progress event for database user operations.
func (d *JobDeps) BroadcastUserProgress(server *servermodels.Server, event, userID, status, message string) {
	d.BroadcastServerEvent(server, event, map[string]any{
		"server_id": server.ID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
