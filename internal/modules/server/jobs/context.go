package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// JobContext holds dependencies for server jobs.
type JobContext struct {
	DB              *gorm.DB
	Repos           contracts.RepositoryRegistry
	Logger          *zerolog.Logger
	WS              broadcast.TeamBroadcaster
	Dispatcher      taskrunner.TaskDispatcher
	ProviderFactory *providers.Factory
	TaskRunnerDeps  *tasks.TaskRunnerDeps
	Queue           *queue.Client
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
		DB:              db,
		Repos:           repos,
		Logger:          logger,
		WS:              ws,
		Dispatcher:      dispatcher,
		ProviderFactory: providerFactory,
		Queue:           queueClient,
		TaskRunnerDeps: &tasks.TaskRunnerDeps{
			DB:         db,
			Queue:      queueClient,
			Dispatcher: dispatcher,
			Logger:     logger,
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

// BroadcastToTeam sends a websocket event to a team channel.
func (c *JobContext) BroadcastToTeam(teamID, event string, data any) {
	if c.WS != nil {
		c.WS.BroadcastToTeam(teamID, event, data)
	}
}

// BroadcastServerEvent broadcasts an event for a server to its team channel.
func (c *JobContext) BroadcastServerEvent(server *models.Server, event string, data any) {
	if c.WS != nil && server != nil {
		c.WS.BroadcastToTeam(server.TeamID, event, data)
	}
}

// LogInfo logs an info message with optional fields.
func (c *JobContext) LogInfo(msg string, fields ...any) {
	if c.Logger == nil {
		return
	}
	event := c.Logger.Info()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogError logs an error message with optional fields.
func (c *JobContext) LogError(err error, msg string, fields ...any) {
	if c.Logger == nil {
		return
	}
	event := c.Logger.Error().Err(err)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}
