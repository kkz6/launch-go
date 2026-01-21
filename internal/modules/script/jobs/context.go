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
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
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
// It embeds pkgjobs.ServerContext for common functionality, typed repository access,
// and server task execution capabilities.
//
// Access common dependencies via inherited methods:
//   - ctx.DB() - database connection
//   - ctx.Logger() - zerolog logger
//   - ctx.WS() - websocket broadcaster
//   - ctx.Queue() - queue client
//   - ctx.Repos() - repository registry (*ScriptRepos)
type JobContext struct {
	*pkgjobs.ServerContext[*ScriptRepos]
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
	scriptRepos := &ScriptRepos{
		script: repos,
		server: serverRepos,
	}

	deps := pkgjobs.ServerContextDeps{
		BaseDeps: pkgjobs.BaseDeps{
			DB:         db,
			Logger:     logger,
			WS:         ws,
			Dispatcher: dispatcher,
			Queue:      queueClient,
		},
	}

	serverCtx := pkgjobs.NewServerContext(deps, scriptRepos)

	return &JobContext{
		ServerContext:  serverCtx,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{ServerTaskDeps: serverCtx.TaskDeps()},
	}
}

// RunTaskOnServer creates a TaskRunner for executing a task on a server.
func (c *JobContext) RunTaskOnServer(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return c.TaskRunnerDeps.NewRunner(server, task)
}
