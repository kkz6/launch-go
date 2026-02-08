package jobs

import (
	"github.com/hibiken/asynq"

	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gitrepos "github.com/kkz6/launch-go/internal/modules/git/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var deps *JobDeps

// Register initializes and registers all site job handlers.
func Register(
	mux *asynq.ServeMux,
	appDeps app.Deps,
	repos *repositories.Registry,
	serverRepos *serverrepos.Registry,
	sourceControlRepo *gitrepos.SourceControlRepository,
	providerFactory *gitproviders.ProviderFactory,
) {
	deps = NewJobDeps(appDeps, repos, serverRepos, sourceControlRepo, providerFactory)
	registerHandlers(mux)
}

func registerHandlers(mux *asynq.ServeMux) {
	// Command jobs
	pkgjobs.RegisterTyped(mux, TypeRunCommand, NewRunCommandJob)

	// Deployment jobs
	pkgjobs.RegisterTyped(mux, TypeDeploy, NewDeployJob)
	pkgjobs.RegisterTyped(mux, TypeDeployZeroDowntime, NewDeployZeroDowntimeJob)
	pkgjobs.RegisterTyped(mux, TypeRollback, NewRollbackJob)

	// SSL jobs
	pkgjobs.RegisterTyped(mux, TypeInstallSSL, NewInstallSSLJob)

	// Caddyfile jobs
	pkgjobs.RegisterTyped(mux, TypeInstallCaddyfile, NewInstallCaddyfileJob)
	pkgjobs.RegisterTyped(mux, TypeUpdateCaddyfile, NewUpdateCaddyfileJob)
	pkgjobs.RegisterTyped(mux, TypeUninstallCaddyfile, NewUninstallCaddyfileJob)

	// Queue jobs
	pkgjobs.RegisterTyped(mux, TypeSyncQueues, NewSyncQueuesJob)
	pkgjobs.RegisterTyped(mux, TypeSyncAllQueues, NewSyncAllQueuesJob)
	pkgjobs.RegisterTyped(mux, TypeInstallQueue, NewInstallQueueJob)
	pkgjobs.RegisterTyped(mux, TypeRestartQueue, NewRestartQueueJob)
	pkgjobs.RegisterTyped(mux, TypeRestartAllSiteQueues, NewRestartAllSiteQueuesJob)
	pkgjobs.RegisterTyped(mux, TypeUninstallQueue, NewUninstallQueueJob)

	// Site uninstallation
	pkgjobs.RegisterTyped(mux, TypeUninstallSite, NewUninstallSiteJob)

	// Laravel feature jobs
	pkgjobs.RegisterTyped(mux, TypeEnableLaravelScheduler, NewEnableLaravelSchedulerJob)
	pkgjobs.RegisterTyped(mux, TypeDisableLaravelScheduler, NewDisableLaravelSchedulerJob)
	pkgjobs.RegisterTyped(mux, TypeEnableLaravelQueue, NewEnableLaravelQueueJob)
	pkgjobs.RegisterTyped(mux, TypeDisableLaravelQueue, NewDisableLaravelQueueJob)
	pkgjobs.RegisterTyped(mux, TypeEnableLaravelHorizon, NewEnableLaravelHorizonJob)
	pkgjobs.RegisterTyped(mux, TypeDisableLaravelHorizon, NewDisableLaravelHorizonJob)
	pkgjobs.RegisterTyped(mux, TypeAnalyzeLaravelFeatures, NewAnalyzeLaravelFeaturesJob)
	pkgjobs.RegisterTyped(mux, TypeEnableLaravelInertia, NewEnableLaravelInertiaJob)
	pkgjobs.RegisterTyped(mux, TypeDisableLaravelInertia, NewDisableLaravelInertiaJob)

	// WordPress jobs
	pkgjobs.RegisterTyped(mux, TypeInstallWordpressCron, NewInstallWordpressCronJob)

	// Deployment management jobs
	pkgjobs.RegisterTyped(mux, TypeCreateDeployment, NewCreateDeploymentJob)
	pkgjobs.RegisterTyped(mux, TypeCleanupPendingSiteDeployment, NewCleanupPendingSiteDeploymentJob)

	// Site configuration jobs
	pkgjobs.RegisterTyped(mux, TypeUpdateSiteTLSSetting, NewUpdateSiteTLSSettingJob)
}
