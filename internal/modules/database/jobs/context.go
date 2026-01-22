package jobs

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

var jobContext *JobContext

// JobContext holds dependencies for database job execution.
// It embeds pkgjobs.ServerContext for common functionality, task execution,
// and typed repository access.
//
// Access common dependencies via inherited methods:
//   - ctx.DB() - database connection
//   - ctx.Logger() - zerolog logger
//   - ctx.WS() - websocket broadcaster
//   - ctx.Queue() - queue client
//   - ctx.Repos() - repository registry
type JobContext struct {
	*pkgjobs.ServerContext[*repositories.Registry]
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

// NewJobContext creates a new database job context.
func NewJobContext(
	db *gorm.DB,
	repos *repositories.Registry,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
	dispatcher taskrunner.TaskDispatcher,
	queueClient *queue.Client,
) *JobContext {
	deps := pkgjobs.ServerContextDeps{
		BaseDeps: pkgjobs.BaseDeps{
			DB:         db,
			Logger:     logger,
			WS:         ws,
			Dispatcher: dispatcher,
			Queue:      queueClient,
		},
	}

	serverCtx := pkgjobs.NewServerContext(deps, repos)

	return &JobContext{
		ServerContext:  serverCtx,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{ServerTaskDeps: serverCtx.TaskDeps()},
	}
}

// SetJobContext sets the global job context.
func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

// GetJobContext returns the global job context.
func GetJobContext() *JobContext {
	return jobContext
}

// RunTaskOnServer creates a TaskRunner for executing a task on a server.
func (c *JobContext) RunTaskOnServer(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return c.TaskRunnerDeps.NewRunner(server, task)
}

// BroadcastDatabaseEvent broadcasts a database event to a team channel.
func (c *JobContext) BroadcastDatabaseEvent(server *servermodels.Server, event string, data any) {
	if server != nil {
		c.ServerContext.BroadcastServerEvent(server.TeamID, event, data)
	}
}

// GetServer returns a server by ID.
// Note: This accesses the server module's data directly as a cross-module operation.
func (c *JobContext) GetServer(ctx context.Context, serverID string) (*servermodels.Server, error) {
	return repository.Find[servermodels.Server](ctx, c.DB(), serverID)
}

// GetServerWithServices returns a server by ID with services preloaded.
// Note: This accesses the server module's data directly as a cross-module operation.
func (c *JobContext) GetServerWithServices(ctx context.Context, serverID string) (*servermodels.Server, error) {
	return repository.NewQuery[servermodels.Server](ctx, c.DB()).
		Preload("Services").
		FindByID(serverID).
		FirstOrFail()
}

// GetDatabaseType returns the database type for a server (mysql or postgresql).
func (c *JobContext) GetDatabaseType(ctx context.Context, serverID string) string {
	service := c.getDatabaseService(ctx, serverID)
	if service == nil {
		return "mysql"
	}

	if service.Type == servertypes.ServiceTypePostgreSQL {
		return "postgresql"
	}
	return "mysql"
}

// GetDatabaseServiceType returns the database service type for a server.
func (c *JobContext) GetDatabaseServiceType(ctx context.Context, serverID string) servertypes.ServiceType {
	service := c.getDatabaseService(ctx, serverID)
	if service == nil {
		return servertypes.ServiceTypeMySQL
	}
	return service.Type
}

// getDatabaseService returns the database service for a server (internal helper).
func (c *JobContext) getDatabaseService(ctx context.Context, serverID string) *servermodels.InstalledService {
	service, err := repository.NewQuery[servermodels.InstalledService](ctx, c.DB()).
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
func (c *JobContext) GetTaskFactory(ctx context.Context, serverID string) *tasks.Factory {
	dbType := c.GetDatabaseServiceType(ctx, serverID)
	return tasks.NewFactory(dbType)
}

// BroadcastDatabaseProgress broadcasts a standard progress event for database operations.
func (c *JobContext) BroadcastDatabaseProgress(server *servermodels.Server, event, entityID, status, message string) {
	data := map[string]any{
		"server_id": server.ID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	if entityID != "" {
		data["database_id"] = entityID
	}
	c.BroadcastDatabaseEvent(server, event, data)
}

// BroadcastUserProgress broadcasts a standard progress event for database user operations.
func (c *JobContext) BroadcastUserProgress(server *servermodels.Server, event, userID, status, message string) {
	c.BroadcastDatabaseEvent(server, event, map[string]any{
		"server_id": server.ID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
