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

// JobContext holds dependencies for server jobs.
// It embeds pkgjobs.Base for common logging and broadcasting functionality.
type JobContext struct {
	pkgjobs.Base
	// Public fields for backward compatibility with existing jobs
	DB              *gorm.DB
	Repos           contracts.RepositoryRegistry
	Logger          *zerolog.Logger
	WS              broadcast.TeamBroadcaster
	Queue           *queue.Client
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
	return &JobContext{
		Base: pkgjobs.NewBase(pkgjobs.BaseDeps{
			DB:         db,
			Logger:     logger,
			WS:         ws,
			Dispatcher: dispatcher,
			Queue:      queueClient,
		}),
		// Public fields for backward compatibility
		DB:              db,
		Repos:           repos,
		Logger:          logger,
		WS:              ws,
		Queue:           queueClient,
		ProviderFactory: providerFactory,
		TaskRunnerDeps: &tasks.TaskRunnerDeps{
			DB:          db,
			Queue:       queueClient,
			Dispatcher:  dispatcher,
			Logger:      logger,
			Broadcaster: ws,
		},
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
	if c.WS != nil && server != nil {
		c.WS.BroadcastToTeam(server.TeamID, event, data)
	}
}
