package jobs

import (
	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var jobContext *JobContext

// SetJobContext sets the global job context
func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

// RegisterHandlers registers all site job handlers with the asynq mux.
func RegisterHandlers(mux *asynq.ServeMux) {
	// Command jobs
	pkgjobs.RegisterHandler(mux, TypeRunCommand, jobContext, NewRunCommandJob)

	// Deployment jobs
	pkgjobs.RegisterHandler(mux, TypeDeploy, jobContext, NewDeployJob)
	pkgjobs.RegisterHandler(mux, TypeDeployZeroDowntime, jobContext, NewDeployZeroDowntimeJob)
	pkgjobs.RegisterHandler(mux, TypeRollback, jobContext, NewRollbackJob)

	// SSL jobs
	pkgjobs.RegisterHandler(mux, TypeInstallSSL, jobContext, NewInstallSSLJob)

	// Caddyfile jobs
	pkgjobs.RegisterHandler(mux, TypeInstallCaddyfile, jobContext, NewInstallCaddyfileJob)
	pkgjobs.RegisterHandler(mux, TypeUpdateCaddyfile, jobContext, NewUpdateCaddyfileJob)
	pkgjobs.RegisterHandler(mux, TypeUninstallCaddyfile, jobContext, NewUninstallCaddyfileJob)

	// Queue jobs
	pkgjobs.RegisterHandler(mux, TypeSyncQueues, jobContext, NewSyncQueuesJob)
	pkgjobs.RegisterHandler(mux, TypeInstallQueue, jobContext, NewInstallQueueJob)
	pkgjobs.RegisterHandler(mux, TypeRestartQueue, jobContext, NewRestartQueueJob)
	pkgjobs.RegisterHandler(mux, TypeUninstallQueue, jobContext, NewUninstallQueueJob)

	// Site uninstallation
	pkgjobs.RegisterHandler(mux, TypeUninstallSite, jobContext, NewUninstallSiteJob)

	// Laravel feature jobs
	pkgjobs.RegisterHandler(mux, TypeEnableLaravelScheduler, jobContext, NewEnableLaravelSchedulerJob)
	pkgjobs.RegisterHandler(mux, TypeDisableLaravelScheduler, jobContext, NewDisableLaravelSchedulerJob)
	pkgjobs.RegisterHandler(mux, TypeEnableLaravelQueue, jobContext, NewEnableLaravelQueueJob)
	pkgjobs.RegisterHandler(mux, TypeDisableLaravelQueue, jobContext, NewDisableLaravelQueueJob)
	pkgjobs.RegisterHandler(mux, TypeEnableLaravelHorizon, jobContext, NewEnableLaravelHorizonJob)
	pkgjobs.RegisterHandler(mux, TypeDisableLaravelHorizon, jobContext, NewDisableLaravelHorizonJob)
}
