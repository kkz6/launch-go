package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	servercontracts "github.com/kkz6/launch-go/internal/modules/server/contracts"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/module"
)

var jobContext *JobContext

// Register initializes and registers all backup job handlers
func Register(mux *asynq.ServeMux, deps module.Deps, repos *repositories.Registry, serverRepos servercontracts.RepositoryRegistry) {
	jobContext = NewJobContext(
		deps.DB,
		repos,
		serverRepos,
		deps.Logger,
		deps.Queue,
	)

	registerHandlers(mux)
}

// registerHandlers registers all backup job handlers with the asynq mux
func registerHandlers(mux *asynq.ServeMux) {
	pkgjobs.RegisterHandler(mux, TypeInstallBackup, jobContext, NewInstallBackupJob)
	pkgjobs.RegisterHandler(mux, TypeDeleteBackup, jobContext, NewDeleteBackupJob)
	pkgjobs.RegisterHandler(mux, TypeRunManualBackup, jobContext, NewRunManualBackupJob)
	pkgjobs.RegisterHandler(mux, TypeSyncServerLaunchConfig, jobContext, NewSyncServerLaunchConfigJob)
}
