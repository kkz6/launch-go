package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// ScriptRepos combines script and server repositories for the script module.
type ScriptRepos struct {
	script *repositories.Registry
	server *serverrepos.Registry
}

// Script returns the script repository.
func (r *ScriptRepos) Script() *repositories.ScriptRepository {
	return r.script.Script()
}

// Execution returns the execution repository.
func (r *ScriptRepos) Execution() *repositories.ScriptExecutionRepository {
	return r.script.Execution()
}

// Server returns the server repository registry.
func (r *ScriptRepos) Server() *serverrepos.Registry {
	return r.server
}

// JobContext holds dependencies for script job execution.
// It embeds pkgjobs.ModuleContext for common functionality and typed repository access.
type JobContext struct {
	*pkgjobs.ModuleContext[*ScriptRepos]
	// Public fields for backward compatibility with existing jobs
	DB             *gorm.DB
	Repos          *ScriptRepos
	Logger         *zerolog.Logger
	WS             broadcast.TeamBroadcaster
	Queue          *queue.Client
	ServerRepos    *serverrepos.Registry
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

// NewJobContext creates a new script job context.
func NewJobContext(
	db *gorm.DB,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
	dispatcher taskrunner.TaskDispatcher,
	queueClient *queue.Client,
	repos *repositories.Registry,
	serverRepos *serverrepos.Registry,
) *JobContext {
	taskRunnerDeps := &servertasks.TaskRunnerDeps{
		DB:          db,
		Queue:       queueClient,
		Dispatcher:  dispatcher,
		Logger:      logger,
		Broadcaster: ws,
	}

	scriptRepos := &ScriptRepos{
		script: repos,
		server: serverRepos,
	}

	return &JobContext{
		ModuleContext: pkgjobs.NewModuleContext(pkgjobs.BaseDeps{
			DB:         db,
			Logger:     logger,
			WS:         ws,
			Dispatcher: dispatcher,
			Queue:      queueClient,
		}, scriptRepos),
		// Public fields for backward compatibility
		DB:             db,
		Repos:          scriptRepos,
		Logger:         logger,
		WS:             ws,
		Queue:          queueClient,
		ServerRepos:    serverRepos,
		TaskRunnerDeps: taskRunnerDeps,
	}
}

// RunTaskOnServer creates a TaskRunner for executing a task on a server.
func (c *JobContext) RunTaskOnServer(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return c.TaskRunnerDeps.NewRunner(server, task)
}
