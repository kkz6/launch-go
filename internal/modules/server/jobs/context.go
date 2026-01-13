package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// JobContext holds all dependencies needed by server jobs.
// This is similar to Laravel's dependency injection for jobs.
type JobContext struct {
	DB         *gorm.DB
	Repo       contracts.Repository
	Logger     *zerolog.Logger
	WS         jobs.Broadcaster
	Dispatcher *taskrunner.Dispatcher

	// TaskRunner dependencies for executing tasks on servers
	TaskRunnerDeps *tasks.TaskRunnerDeps
}

// NewJobContext creates a new job context with all dependencies
func NewJobContext(
	db *gorm.DB,
	repo contracts.Repository,
	logger *zerolog.Logger,
	ws jobs.Broadcaster,
	dispatcher *taskrunner.Dispatcher,
) *JobContext {
	return &JobContext{
		DB:         db,
		Repo:       repo,
		Logger:     logger,
		WS:         ws,
		Dispatcher: dispatcher,
		TaskRunnerDeps: &tasks.TaskRunnerDeps{
			DB:         db,
			Dispatcher: dispatcher,
			Logger:     logger,
		},
	}
}

// ServerTaskRunner is a helper to create TaskRunners for a server.
// It provides a fluent interface similar to Laravel's $server->runTask().
type ServerTaskRunner struct {
	ctx    *JobContext
	server *models.Server
}

// ForServer creates a new ServerTaskRunner for the given server
func (c *JobContext) ForServer(server *models.Server) *ServerTaskRunner {
	return &ServerTaskRunner{
		ctx:    c,
		server: server,
	}
}

// RunTask creates a TaskRunner for the given task with a fluent interface.
// Usage:
//
//	ctx.ForServer(server).RunTask(task).AsRoot().TrackInDB().Dispatch(ctx)
func (s *ServerTaskRunner) RunTask(task taskrunner.Task) *tasks.TaskRunner {
	return s.ctx.TaskRunnerDeps.NewRunner(s.server, task)
}

// ServerJobBase provides common functionality for server jobs.
// Embed this in your job structs instead of jobs.BaseJob.
type ServerJobBase struct {
	jobs.BaseJob
	Ctx *JobContext
}

// SetContext sets the job context with all dependencies
func (j *ServerJobBase) SetContext(ctx *JobContext) {
	j.Ctx = ctx
	j.DB = ctx.DB
	j.Logger = ctx.Logger
	j.WS = ctx.WS
}

// RunTaskOnServer creates a TaskRunner for executing a task on a server.
// This is the primary way jobs should execute tasks.
//
// Usage:
//
//	result, err := j.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
func (j *ServerJobBase) RunTaskOnServer(server *models.Server, task taskrunner.Task) *tasks.TaskRunner {
	return j.Ctx.TaskRunnerDeps.NewRunner(server, task)
}

// Repo returns the repository for database operations
func (j *ServerJobBase) Repo() contracts.Repository {
	return j.Ctx.Repo
}

// BroadcastServerEvent broadcasts an event to a server's channel
func (j *ServerJobBase) BroadcastServerEvent(serverID, event string, data interface{}) {
	if j.WS != nil {
		j.WS.BroadcastToServer(serverID, event, data)
	}
}

// ContextSetter is an interface for jobs that need context injection
type ContextSetter interface {
	SetContext(ctx *JobContext)
}
