package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gitrepos "github.com/kkz6/launch-go/internal/modules/git/repositories"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// JobDeps holds all dependencies for site jobs.
type JobDeps struct {
	*pkgjobs.Deps
	Repos             *repositories.Registry
	ServerRepos       *serverrepos.Registry
	SourceControlRepo *gitrepos.SourceControlRepository
	ProviderFactory   *gitproviders.ProviderFactory
	FrontendURL       string
	TaskRunnerDeps    *servertasks.TaskRunnerDeps
}

// NewJobDeps creates a new JobDeps from app dependencies.
func NewJobDeps(
	appDeps app.Deps,
	repos *repositories.Registry,
	serverRepos *serverrepos.Registry,
	sourceControlRepo *gitrepos.SourceControlRepository,
	providerFactory *gitproviders.ProviderFactory,
) *JobDeps {
	jobDeps := &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          appDeps.DB,
			Logger:      appDeps.Logger,
			Queue:       appDeps.Queue,
			Broadcaster: appDeps.WebSocket,
			Dispatcher:  appDeps.Dispatcher,
		},
		Repos:             repos,
		ServerRepos:       serverRepos,
		SourceControlRepo: sourceControlRepo,
		ProviderFactory:   providerFactory,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:          appDeps.DB,
			Queue:       appDeps.Queue,
			Dispatcher:  appDeps.Dispatcher,
			Logger:      appDeps.Logger,
			Broadcaster: appDeps.WebSocket,
			Notifier:    appDeps.Notifier,
		},
	}
	if appDeps.Config != nil {
		jobDeps.FrontendURL = appDeps.Config.App.Frontend()
	}
	return jobDeps
}

// NewJobDepsWithParams creates JobDeps with explicit parameters (for testing or custom setup).
func NewJobDepsWithParams(
	db *gorm.DB,
	repos *repositories.Registry,
	serverRepos *serverrepos.Registry,
	sourceControlRepo *gitrepos.SourceControlRepository,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
	dispatcher taskrunner.TaskDispatcher,
	providerFactory *gitproviders.ProviderFactory,
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
		Repos:             repos,
		ServerRepos:       serverRepos,
		SourceControlRepo: sourceControlRepo,
		ProviderFactory:   providerFactory,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:          db,
			Queue:       queueClient,
			Dispatcher:  dispatcher,
			Logger:      logger,
			Broadcaster: ws,
		},
	}
}

// RunTask creates a task runner for a server with a task.
func (d *JobDeps) RunTask(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return d.TaskRunnerDeps.NewRunner(server, task)
}

// BroadcastServerEvent broadcasts an event for a server to its team channel.
func (d *JobDeps) BroadcastServerEvent(server *servermodels.Server, event string, data any) {
	if server != nil {
		d.BroadcastToTeam(server.TeamID, event, data)
	}
}
