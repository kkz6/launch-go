package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ServerTaskDeps holds dependencies needed to run tasks on servers.
// This struct is designed to be embedded by module-specific TaskRunnerDeps
// to avoid field duplication.
//
// Example embedding:
//
//	type TaskRunnerDeps struct {
//	    jobs.ServerTaskDeps
//	}
type ServerTaskDeps struct {
	DB          *gorm.DB
	Queue       *queue.Client
	Dispatcher  taskrunner.TaskDispatcher
	Logger      *zerolog.Logger
	Broadcaster broadcast.TeamBroadcaster
	Notifier    taskrunner.NotifierService
	LocalMode   bool
}

// NewServerTaskDeps creates ServerTaskDeps from BaseDeps.
// This eliminates the repeated construction of task runner dependencies
// across modules (site, server, database).
func NewServerTaskDeps(deps BaseDeps) ServerTaskDeps {
	return ServerTaskDeps{
		DB:          deps.DB,
		Queue:       deps.Queue,
		Dispatcher:  deps.Dispatcher,
		Logger:      deps.Logger,
		Broadcaster: deps.WS,
	}
}

// WithLocalMode returns a copy of ServerTaskDeps with LocalMode set.
func (d ServerTaskDeps) WithLocalMode(local bool) ServerTaskDeps {
	d.LocalMode = local
	return d
}

// ServerContextDeps holds all dependencies for creating a ServerContext.
type ServerContextDeps struct {
	BaseDeps
	LocalMode bool
}

// ServerContext extends ModuleContext with server task execution capabilities.
// Use this for modules that need to run SSH tasks on servers (site, server, database).
//
// Example:
//
//	type JobContext struct {
//	    *jobs.ServerContext[*repositories.Registry]
//	    ProviderFactory *providers.Factory
//	}
//
//	func NewJobContext(deps jobs.ServerContextDeps, repos *repositories.Registry) *JobContext {
//	    return &JobContext{
//	        ServerContext: jobs.NewServerContext(deps, repos),
//	    }
//	}
type ServerContext[R any] struct {
	*ModuleContext[R]
	taskDeps ServerTaskDeps
}

// NewServerContext creates a new ServerContext with task execution capabilities.
func NewServerContext[R any](deps ServerContextDeps, repos R) *ServerContext[R] {
	return &ServerContext[R]{
		ModuleContext: NewModuleContext(deps.BaseDeps, repos),
		taskDeps:      NewServerTaskDeps(deps.BaseDeps).WithLocalMode(deps.LocalMode),
	}
}

// TaskDeps returns the server task dependencies.
// These can be embedded directly into module-specific TaskRunnerDeps.
func (c *ServerContext[R]) TaskDeps() ServerTaskDeps {
	return c.taskDeps
}

// BroadcastServerEvent broadcasts an event for a server to its team channel.
// This is a convenience method that handles nil checks.
//
// Example:
//
//	ctx.BroadcastServerEvent(server.TeamID, "server.updated", data)
func (c *ServerContext[R]) BroadcastServerEvent(teamID, event string, data any) {
	if c.WS() != nil && teamID != "" {
		c.WS().BroadcastToTeam(teamID, event, data)
	}
}
