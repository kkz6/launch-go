package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/module"
	"github.com/kkz6/launch-go/internal/pkg/sshkey"
	"github.com/kkz6/launch-go/internal/queue"
)

var jobContext *JobContext

// Register initializes and registers all server job handlers.
func Register(mux *asynq.ServeMux, deps module.Deps, repos contracts.RepositoryRegistry) {
	providerFactory := providers.NewFactory(sshkey.NewGenerator())

	jobContext = NewJobContext(
		deps.DB,
		repos,
		deps.Logger,
		deps.WebSocket,
		deps.Dispatcher,
		providerFactory,
		deps.Queue,
	)

	registerHandlers(mux)
}

// registerHandlers registers all server job handlers with the asynq mux.
func registerHandlers(mux *asynq.ServeMux) {
	// Server lifecycle jobs
	pkgjobs.RegisterHandler(mux, TypeCreateOnProvider, jobContext, NewCreateOnProviderJob)
	pkgjobs.RegisterHandler(mux, TypeWaitForServerToConnect, jobContext, NewWaitForServerToConnectJob)
	pkgjobs.RegisterHandler(mux, TypeProvisionServer, jobContext, NewProvisionServerJob)
	pkgjobs.RegisterHandler(mux, TypeDeleteServer, jobContext, NewDeleteServerJob)
	pkgjobs.RegisterHandler(mux, TypeCleanupFailedProvisioning, jobContext, NewCleanupFailedProvisioningJob)
	pkgjobs.RegisterHandler(mux, TypeRebootServer, jobContext, NewRebootServerJob)
	pkgjobs.RegisterHandler(mux, TypeArchiveServer, jobContext, NewArchiveServerJob)
	pkgjobs.RegisterHandler(mux, TypeUnarchiveServer, jobContext, NewUnarchiveServerJob)
	pkgjobs.RegisterHandler(mux, TypeUpdateConnectivity, jobContext, NewUpdateConnectivityJob)

	// Cron jobs
	pkgjobs.RegisterHandler(mux, TypeInstallCron, jobContext, NewInstallCronJob)
	pkgjobs.RegisterHandler(mux, TypeUninstallCron, jobContext, NewUninstallCronJob)

	// Daemon jobs
	pkgjobs.RegisterHandler(mux, TypeInstallDaemon, jobContext, NewInstallDaemonJob)
	pkgjobs.RegisterHandler(mux, TypeUninstallDaemon, jobContext, NewUninstallDaemonJob)
	pkgjobs.RegisterHandler(mux, TypeRestartDaemon, jobContext, NewRestartDaemonJob)
	pkgjobs.RegisterHandler(mux, TypeSyncDaemons, jobContext, NewSyncDaemonsJob)

	// Firewall jobs
	pkgjobs.RegisterHandler(mux, TypeInstallFirewallRule, jobContext, NewInstallFirewallRuleJob)
	pkgjobs.RegisterHandler(mux, TypeUninstallFirewall, jobContext, NewUninstallFirewallRuleJob)

	// SSH key jobs
	pkgjobs.RegisterHandler(mux, TypeAddSSHKey, jobContext, NewAddSSHKeyJob)
	pkgjobs.RegisterHandler(mux, TypeRemoveSSHKey, jobContext, NewRemoveSSHKeyJob)

	// Service jobs
	pkgjobs.RegisterHandler(mux, TypeAddService, jobContext, NewAddServiceJob)
	pkgjobs.RegisterHandler(mux, TypeRemoveService, jobContext, NewRemoveServiceJob)
	pkgjobs.RegisterHandler(mux, TypeServiceOperation, jobContext, NewServiceOperationJob)
	pkgjobs.RegisterHandler(mux, TypeCheckServiceStatus, jobContext, NewCheckServiceStatusJob)
	pkgjobs.RegisterHandler(mux, TypeConfigureOpcache, jobContext, NewConfigureOpcacheJob)

	// PHP jobs
	pkgjobs.RegisterHandler(mux, TypeSetDefaultPhp, jobContext, NewSetDefaultPhpJob)
	pkgjobs.RegisterHandler(mux, TypeInstallPhpExtension, jobContext, NewInstallPhpExtensionJob)
	pkgjobs.RegisterHandler(mux, TypeUninstallPhpExtension, jobContext, NewUninstallPhpExtensionJob)
	pkgjobs.RegisterHandler(mux, TypeCleanupFailedPhpInstallation, jobContext, NewCleanupFailedPhpInstallationJob)
	pkgjobs.RegisterHandler(mux, TypeCleanupFailedPhpExtensionInstall, jobContext, NewCleanupFailedPhpExtensionInstallJob)
	pkgjobs.RegisterHandler(mux, TypeCleanupFailedPhpExtensionUninstall, jobContext, NewCleanupFailedPhpExtensionUninstallJob)

	// Security audit jobs
	pkgjobs.RegisterHandler(mux, TypeVulnerabilityAudit, jobContext, NewVulnerabilityAuditJob)

	// Task and maintenance jobs
	pkgjobs.RegisterHandler(mux, TypeCheckDaemonStatus, jobContext, NewCheckDaemonStatusJob)
	pkgjobs.RegisterHandler(mux, TypeUpdateTaskOutput, jobContext, NewUpdateTaskOutputJob)
	pkgjobs.RegisterHandler(mux, TypeUpdateUserPublicKey, jobContext, NewUpdateUserPublicKeyJob)
	pkgjobs.RegisterHandler(mux, TypeInstallTaskCleanupCron, jobContext, NewInstallTaskCleanupCronJob)
	pkgjobs.RegisterHandler(mux, TypeRunAfterUpdate, jobContext, NewRunAfterUpdateJob)

	// Scheduled maintenance jobs
	pkgjobs.RegisterHandler(mux, TypeCleanupOldMetrics, jobContext, NewCleanupOldMetricsJob)
}

// GetScheduledTasks returns the scheduled tasks for the server module
func GetScheduledTasks() []queue.ScheduledTask {
	cleanupTask, _ := NewCleanupOldMetricsTask()

	return []queue.ScheduledTask{
		{
			CronSpec: CleanupOldMetricsCronSpec(),
			Task:     cleanupTask,
			Opts:     []asynq.Option{asynq.Queue("low")},
		},
	}
}
