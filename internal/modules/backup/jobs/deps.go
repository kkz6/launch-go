package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	servercontracts "github.com/kkz6/launch-go/internal/modules/server/contracts"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// JobDeps holds all dependencies for backup jobs.
type JobDeps struct {
	*pkgjobs.Deps
	Repos       *repositories.Registry
	ServerRepos servercontracts.RepositoryRegistry
	// TaskRunnerDeps lets backup jobs build a taskrunner.TaskRunner the
	// same way the docker/server modules do, so a manual backup can run
	// an SSH script with TrackInDB + OnTaskCreated (powers the live log
	// console).
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

// RunTask returns a TaskRunner bound to the given server + task. Same
// affordance as the docker module's JobDeps.RunTask — kept on backup's
// JobDeps so backup jobs don't cross-import server.jobs.JobDeps.
func (d *JobDeps) RunTask(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return d.TaskRunnerDeps.NewRunner(server, task)
}

// NewJobDeps creates a new JobDeps from app dependencies.
func NewJobDeps(appDeps app.Deps, repos *repositories.Registry, serverRepos servercontracts.RepositoryRegistry) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          appDeps.DB,
			Logger:      appDeps.Logger,
			Queue:       appDeps.Queue,
			Broadcaster: appDeps.WebSocket,
			Dispatcher:  appDeps.Dispatcher,
		},
		Repos:       repos,
		ServerRepos: serverRepos,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
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
	repos *repositories.Registry,
	serverRepos servercontracts.RepositoryRegistry,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
	queueClient *queue.Client,
) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          db,
			Logger:      logger,
			Queue:       queueClient,
			Broadcaster: ws,
		},
		Repos:       repos,
		ServerRepos: serverRepos,
	}
}
