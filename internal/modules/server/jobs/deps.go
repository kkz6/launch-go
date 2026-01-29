package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	pkgtaskrunner "github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// JobDeps holds all dependencies for server jobs.
type JobDeps struct {
	*pkgjobs.Deps
	Repos           contracts.RepositoryRegistry
	ProviderFactory *providers.Factory
	TaskRunnerDeps  *tasks.TaskRunnerDeps
}

// NewJobDeps creates a new JobDeps from app dependencies.
func NewJobDeps(appDeps app.Deps, repos contracts.RepositoryRegistry) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          appDeps.DB,
			Logger:      appDeps.Logger,
			Queue:       appDeps.Queue,
			Broadcaster: appDeps.WebSocket,
			Dispatcher:  appDeps.Dispatcher,
		},
		Repos:           repos,
		ProviderFactory: providers.NewFactory(sshkey.NewGenerator()),
		TaskRunnerDeps: &tasks.TaskRunnerDeps{
			DB:          appDeps.DB,
			Queue:       appDeps.Queue,
			Dispatcher:  appDeps.Dispatcher,
			Logger:      appDeps.Logger,
			Broadcaster: appDeps.WebSocket,
			Notifier:    appDeps.Notifier,
		},
	}
}

// NewJobDepsWithParams creates JobDeps with explicit parameters (for testing or custom setup).
func NewJobDepsWithParams(
	db *gorm.DB,
	repos contracts.RepositoryRegistry,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
	dispatcher pkgtaskrunner.TaskDispatcher,
	providerFactory *providers.Factory,
	queueClient *queue.Client,
) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          db,
			Logger:      logger,
			Queue:       queueClient,
			Broadcaster: ws,
			Dispatcher:  dispatcher,
		},
		Repos:           repos,
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

// RunTask creates a task runner for a server with a task.
func (d *JobDeps) RunTask(server *models.Server, task pkgtaskrunner.Task) *tasks.TaskRunner {
	return d.TaskRunnerDeps.NewRunner(server, task)
}

// BroadcastServerEvent broadcasts an event for a server to its team channel.
func (d *JobDeps) BroadcastServerEvent(server *models.Server, event string, data any) {
	if server != nil {
		d.BroadcastToTeam(server.TeamID, event, data)
	}
}
