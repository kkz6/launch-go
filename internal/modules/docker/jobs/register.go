package jobs

import (
	"github.com/hibiken/asynq"

	backuprepos "github.com/kkz6/launch-go/internal/modules/backup/repositories"
	certrepos "github.com/kkz6/launch-go/internal/modules/certificate/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// deps is set once by Register and read by NewDeployApplicationJob and
// any sibling job constructors. Matches the package-var pattern used by
// the script and server modules.
var deps *JobDeps

// Register wires JobDeps and binds every asynq task type this module
// publishes to its handler.
func Register(
	mux *asynq.ServeMux,
	appDeps app.Deps,
	repos *repositories.Registry,
	serverRepos *serverrepos.Registry,
	backupRepos *backuprepos.Registry,
	certRepos *certrepos.Registry,
	gitProviders *gitproviders.ProviderFactory,
) {
	deps = NewJobDeps(appDeps, repos, serverRepos, backupRepos, certRepos, gitProviders)
	pkgjobs.RegisterTyped(mux, TypeDeployApplication, NewDeployApplicationJob)
	pkgjobs.RegisterTyped(mux, TypeRemoveApplication, NewRemoveApplicationJob)
	pkgjobs.RegisterTyped(mux, TypeApplicationLifecycle, NewApplicationLifecycleJob)
	pkgjobs.RegisterTyped(mux, TypeRunApplicationSchedule, NewRunApplicationScheduleJob)
	pkgjobs.RegisterTyped(mux, TypePollDueSchedules, NewPollDueSchedulesJob)
	pkgjobs.RegisterTyped(mux, TypeDeployCompose, NewDeployComposeJob)
	pkgjobs.RegisterTyped(mux, TypeRemoveCompose, NewRemoveComposeJob)
	pkgjobs.RegisterTyped(mux, TypeSyncTraefikConfig, NewSyncTraefikConfigJob)
	pkgjobs.RegisterTyped(mux, TypeSyncComposeTraefikConfig, NewSyncComposeTraefikConfigJob)
	pkgjobs.RegisterTyped(mux, TypeRunDatabase, NewRunDatabaseJob)
	pkgjobs.RegisterTyped(mux, TypeDatabaseLifecycle, NewDatabaseLifecycleJob)
	pkgjobs.RegisterTyped(mux, TypeRunBackup, NewRunBackupJob)
	pkgjobs.RegisterTyped(mux, TypePollDueBackups, NewPollDueBackupsJob)
	pkgjobs.RegisterTyped(mux, TypeGHABootstrapWorkflow, NewGHABootstrapWorkflowJob)
	pkgjobs.RegisterTyped(mux, TypePollGHASteps, NewPollGHAStepsJob)
}
