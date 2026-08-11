package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	databaserepos "github.com/kkz6/launch-go/internal/modules/database/repositories"
	servercontracts "github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var deps *JobDeps

// Register initializes and registers all backup job handlers.
// databaseRepos is optional (nil-tolerant) — when wired, manual backup
// runs dump the linked databases before tarring; when nil, only files
// get backed up.
func Register(
	mux *asynq.ServeMux,
	appDeps app.Deps,
	repos *repositories.Registry,
	serverRepos servercontracts.RepositoryRegistry,
	databaseRepos *databaserepos.Registry,
) {
	deps = NewJobDeps(appDeps, repos, serverRepos, databaseRepos)
	registerHandlers(mux)
}

// registerHandlers registers all backup job handlers with the asynq mux.
func registerHandlers(mux *asynq.ServeMux) {
	pkgjobs.RegisterTyped(mux, TypeInstallBackup, NewInstallBackupJob)
	pkgjobs.RegisterTyped(mux, TypeDeleteBackup, NewDeleteBackupJob)
	pkgjobs.RegisterTyped(mux, TypeRunManualBackup, NewRunManualBackupJob)
	pkgjobs.RegisterTyped(mux, TypePollDueBackups, NewPollDueBackupsJob)
	pkgjobs.RegisterTyped(mux, TypeSyncServerLaunchConfig, NewSyncServerLaunchConfigJob)
}
