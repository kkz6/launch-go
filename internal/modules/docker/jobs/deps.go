// Package jobs holds the asynq job handlers for the docker module. The
// shape mirrors internal/modules/script/jobs: a JobDeps carrier holds the
// repositories + the server-module TaskRunner so the job can SSH into the
// docker host without re-wiring all of taskrunner.
package jobs

import (
	backuprepos "github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// JobDeps holds shared dependencies for docker-module asynq jobs.
type JobDeps struct {
	*pkgjobs.Deps
	Repos       *repositories.Registry
	ServerRepos *serverrepos.Registry
	// BackupRepos exposes the global storage_providers repository. The
	// RunBackup job needs it to load S3 credentials for the database
	// backup target (the docker_database_backups row only carries the
	// FK now, not the creds themselves).
	BackupRepos    *backuprepos.Registry
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

// NewJobDeps wires JobDeps from app-level dependencies. ServerRepos +
// BackupRepos are passed explicitly (not constructed here) so the
// docker module and the rest of the system share the same connection-
// backed registries.
func NewJobDeps(
	appDeps app.Deps,
	repos *repositories.Registry,
	serverRepos *serverrepos.Registry,
	backupRepos *backuprepos.Registry,
) *JobDeps {
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
		BackupRepos: backupRepos,
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

// RunTask returns a TaskRunner bound to the given server + task. Same
// affordance as the server module's RunTask, just plumbed through this
// module's JobDeps so we don't need a cross-module dep on server.JobDeps.
func (d *JobDeps) RunTask(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return d.TaskRunnerDeps.NewRunner(server, task)
}
