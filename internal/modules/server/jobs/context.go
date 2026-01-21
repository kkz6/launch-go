package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// These type aliases ensure the imports are used and provide documentation
// about the inherited types from ModuleContext.
var (
	_ *gorm.DB                  // Used via c.DB()
	_ *zerolog.Logger           // Used via c.Logger()
	_ broadcast.TeamBroadcaster // Used via c.WS()
	_ *queue.Client             // Used via c.Queue()
)

// JobContext holds dependencies for server jobs.
// It embeds pkgjobs.ModuleContext for common functionality and typed repository access.
//
// Access common dependencies via inherited methods:
//   - j.Ctx.DB() - database connection
//   - j.Ctx.Logger() - zerolog logger
//   - j.Ctx.WS() - websocket broadcaster
//   - j.Ctx.Queue() - queue client
//   - j.Ctx.Repos() - repository registry (contracts.RepositoryRegistry)
//
// Module-specific fields:
//   - j.Ctx.ProviderFactory - cloud provider factory
//   - j.Ctx.TaskRunnerDeps - task runner dependencies
type JobContext struct {
	*pkgjobs.ModuleContext[contracts.RepositoryRegistry]
	ProviderFactory *providers.Factory
	TaskRunnerDeps  *tasks.TaskRunnerDeps
}

// NewJobContext creates a new job context with all dependencies.
func NewJobContext(
	db *gorm.DB,
	repos contracts.RepositoryRegistry,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
	dispatcher *taskrunner.Dispatcher,
	providerFactory *providers.Factory,
	queueClient *queue.Client,
) *JobContext {
	taskRunnerDeps := &tasks.TaskRunnerDeps{
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
		ProviderFactory: providerFactory,
		TaskRunnerDeps:  taskRunnerDeps,
	}
}

// ServerTaskRunner provides a fluent interface for running tasks on a server.
type ServerTaskRunner struct {
	ctx    *JobContext
	server *models.Server
}

// ForServer creates a task runner bound to a specific server.
func (c *JobContext) ForServer(server *models.Server) *ServerTaskRunner {
	return &ServerTaskRunner{
		ctx:    c,
		server: server,
	}
}

// RunTask executes a task on the server.
func (s *ServerTaskRunner) RunTask(task taskrunner.Task) *tasks.TaskRunner {
	return s.ctx.TaskRunnerDeps.NewRunner(s.server, task)
}

// BroadcastServerEvent broadcasts an event for a server to its team channel.
func (c *JobContext) BroadcastServerEvent(server *models.Server, event string, data any) {
	if c.WS() != nil && server != nil {
		c.WS().BroadcastToTeam(server.TeamID, event, data)
	}
}
