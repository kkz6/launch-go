package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	servercontracts "github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var deps *JobDeps

// Register initializes and registers all backup job handlers.
func Register(mux *asynq.ServeMux, appDeps app.Deps, repos *repositories.Registry, serverRepos servercontracts.RepositoryRegistry) {
	deps = NewJobDeps(appDeps, repos, serverRepos)
	registerHandlers(mux)
}

// registerHandlers registers all backup job handlers with the asynq mux.
func registerHandlers(mux *asynq.ServeMux) {
	pkgjobs.RegisterTyped(mux, TypeInstallBackup, NewInstallBackupJob)
	pkgjobs.RegisterTyped(mux, TypeDeleteBackup, NewDeleteBackupJob)
	pkgjobs.RegisterTyped(mux, TypeRunManualBackup, NewRunManualBackupJob)
	pkgjobs.RegisterTyped(mux, TypeSyncServerLaunchConfig, NewSyncServerLaunchConfigJob)
}
