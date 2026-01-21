package jobs

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	serverenums "github.com/kkz6/launch-go/internal/modules/server/enums"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

var jobContext *JobContext

// JobContext holds dependencies for database job execution.
// It embeds pkgjobs.ModuleContext for common functionality and typed repository access.
type JobContext struct {
	*pkgjobs.ModuleContext[*repositories.Registry]
	// Public fields for backward compatibility with existing jobs
	DB             *gorm.DB
	Logger         *zerolog.Logger
	WS             broadcast.TeamBroadcaster
	Queue          *queue.Client
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
	taskRunnerDeps := &servertasks.TaskRunnerDeps{
		DB:          db,
		Queue:       queueClient,
		Dispatcher:  dispatcher,
		Logger:      logger,
		Broadcaster: ws,
	}

	return &JobContext{
		ModuleContext: pkgjobs.NewModuleContext(pkgjobs.BaseDeps{
			DB:         db,
			Logger:     logger,
			WS:         ws,
			Dispatcher: dispatcher,
			Queue:      queueClient,
		}, repos),
		// Public fields for backward compatibility
		DB:             db,
		Logger:         logger,
		WS:             ws,
		Queue:          queueClient,
		TaskRunnerDeps: taskRunnerDeps,
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
	if c.WS != nil && server != nil {
		c.WS.BroadcastToTeam(server.TeamID, event, data)
	}
}

// GetDatabaseType returns the database type for a server (mysql or postgresql).
func (c *JobContext) GetDatabaseType(ctx context.Context, serverID string) string {
	service, err := repository.NewQuery[servermodels.InstalledService](ctx, c.DB).
		Where("server_id = ? AND type IN ?", serverID, []string{
			string(serverenums.ServiceTypeMySql),
			string(serverenums.ServiceTypePostgreSql),
		}).
		First()

	if err != nil {
		return "mysql"
	}

	if service.Type == serverenums.ServiceTypePostgreSql {
		return "postgresql"
	}
	return "mysql"
}

// GetDatabaseServiceType returns the database service type for a server.
func (c *JobContext) GetDatabaseServiceType(ctx context.Context, serverID string) serverenums.ServiceType {
	service, err := repository.NewQuery[servermodels.InstalledService](ctx, c.DB).
		Where("server_id = ? AND type IN ?", serverID, []string{
			string(serverenums.ServiceTypeMySql),
			string(serverenums.ServiceTypePostgreSql),
		}).
		First()

	if err != nil {
		return serverenums.ServiceTypeMySql
	}

	return service.Type
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
