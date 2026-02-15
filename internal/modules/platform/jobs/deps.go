package jobs

import (
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"

	"github.com/kkz6/launch-go/internal/modules/platform/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// JobDeps holds all dependencies for platform jobs
type JobDeps struct {
	*pkgjobs.Deps
	Repos          *repositories.Registry
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

// NewJobDeps creates a new JobDeps from app dependencies
func NewJobDeps(appDeps app.Deps, repos *repositories.Registry) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          appDeps.DB,
			Logger:      appDeps.Logger,
			Queue:       appDeps.Queue,
			Broadcaster: appDeps.WebSocket,
			Dispatcher:  appDeps.Dispatcher,
		},
		Repos: repos,
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

// RunTask creates a task runner for a server with a task
func (d *JobDeps) RunTask(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return d.TaskRunnerDeps.NewRunner(server, task)
}
