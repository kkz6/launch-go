package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// jobContext is the package-level job context, set during module initialization.
var jobContext *JobContext

// JobContext holds all dependencies needed by database jobs.
// This is similar to the server module's JobContext pattern.
type JobContext struct {
	DB         *gorm.DB
	Repo       *repositories.Repository
	Logger     *zerolog.Logger
	WS         jobs.Broadcaster
	Dispatcher *taskrunner.Dispatcher
	Queue      *queue.Client

	// TaskRunner dependencies for executing tasks on servers
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

// NewJobContext creates a new job context with all dependencies.
func NewJobContext(
	db *gorm.DB,
	repo *repositories.Repository,
	logger *zerolog.Logger,
	ws jobs.Broadcaster,
	dispatcher *taskrunner.Dispatcher,
	queueClient *queue.Client,
) *JobContext {
	return &JobContext{
		DB:         db,
		Repo:       repo,
		Logger:     logger,
		WS:         ws,
		Dispatcher: dispatcher,
		Queue:      queueClient,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:         db,
			Queue:      queueClient,
			Dispatcher: dispatcher,
			Logger:     logger,
		},
	}
}

// SetJobContext sets the package-level job context.
// This must be called during module initialization before jobs are processed.
func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

// GetJobContext returns the package-level job context.
func GetJobContext() *JobContext {
	return jobContext
}

// DatabaseJobBase provides common functionality for database jobs.
// Embed this in your job structs instead of jobs.BaseJob.
//
// Usage:
//
//	type InstallDatabaseJob struct {
//	    DatabaseJobBase
//	    Payload InstallDatabasePayload
//	}
type DatabaseJobBase struct {
	jobs.BaseJob
	Ctx *JobContext
}

// SetContext sets the job context with all dependencies.
func (j *DatabaseJobBase) SetContext(ctx *JobContext) {
	j.Ctx = ctx
	j.DB = ctx.DB
	j.Logger = ctx.Logger
	j.WS = ctx.WS
	j.Dispatcher = ctx.Dispatcher
	j.Queue = ctx.Queue
}

// Repo returns the repository for database operations.
func (j *DatabaseJobBase) Repo() *repositories.Repository {
	return j.Ctx.Repo
}

// RunTaskOnServer creates a TaskRunner for executing a task on a server.
// This is the primary way database jobs should execute tasks.
//
// Usage:
//
//	result, err := j.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
func (j *DatabaseJobBase) RunTaskOnServer(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return j.Ctx.TaskRunnerDeps.NewRunner(server, task)
}

// BroadcastDatabaseEvent broadcasts an event to a server's database channel.
func (j *DatabaseJobBase) BroadcastDatabaseEvent(serverID, event string, data any) {
	if j.WS != nil {
		j.WS.BroadcastToServer(serverID, event, data)
	}
}

// ContextSetter is an interface for jobs that need context injection.
type ContextSetter interface {
	SetContext(ctx *JobContext)
}
