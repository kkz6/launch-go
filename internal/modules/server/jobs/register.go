package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var deps *JobDeps

// Register initializes and registers all server job handlers.
func Register(mux *asynq.ServeMux, appDeps app.Deps, repos contracts.RepositoryRegistry) {
	deps = NewJobDeps(appDeps, repos)
	registerHandlers(mux)
}

func registerHandlers(mux *asynq.ServeMux) {
	// Server lifecycle jobs
	pkgjobs.RegisterTyped(mux, TypeCreateOnProvider, NewCreateOnProviderJob)
	pkgjobs.RegisterTyped(mux, TypeWaitForServerToConnect, NewWaitForServerToConnectJob)
	pkgjobs.RegisterTyped(mux, TypeProvisionServer, NewProvisionServerJob)
	pkgjobs.RegisterTyped(mux, TypeDeleteServer, NewDeleteServerJob)
	pkgjobs.RegisterTyped(mux, TypeCleanupFailedProvisioning, NewCleanupFailedProvisioningJob)
	pkgjobs.RegisterTyped(mux, TypeRebootServer, NewRebootServerJob)
	pkgjobs.RegisterTyped(mux, TypeArchiveServer, NewArchiveServerJob)
	pkgjobs.RegisterTyped(mux, TypeUnarchiveServer, NewUnarchiveServerJob)
	pkgjobs.RegisterTyped(mux, TypeUpdateConnectivity, NewUpdateConnectivityJob)

	// Cron jobs
	pkgjobs.RegisterTyped(mux, TypeInstallCron, NewInstallCronJob)
	pkgjobs.RegisterTyped(mux, TypeUninstallCron, NewUninstallCronJob)

	// Daemon jobs
	pkgjobs.RegisterTyped(mux, TypeInstallDaemon, NewInstallDaemonJob)
	pkgjobs.RegisterTyped(mux, TypeUninstallDaemon, NewUninstallDaemonJob)
	pkgjobs.RegisterTyped(mux, TypeRestartDaemon, NewRestartDaemonJob)
	pkgjobs.RegisterTyped(mux, TypeSyncDaemons, NewSyncDaemonsJob)

	// Firewall jobs
	pkgjobs.RegisterTyped(mux, TypeInstallFirewallRule, NewInstallFirewallRuleJob)
	pkgjobs.RegisterTyped(mux, TypeUninstallFirewall, NewUninstallFirewallRuleJob)

	// SSH key jobs
	pkgjobs.RegisterTyped(mux, TypeAddSSHKey, NewAddSSHKeyJob)
	pkgjobs.RegisterTyped(mux, TypeRemoveSSHKey, NewRemoveSSHKeyJob)

	// Service jobs
	pkgjobs.RegisterTyped(mux, TypeAddService, NewAddServiceJob)
	pkgjobs.RegisterTyped(mux, TypeRemoveService, NewRemoveServiceJob)
	pkgjobs.RegisterTyped(mux, TypeServiceOperation, NewServiceOperationJob)
	pkgjobs.RegisterTyped(mux, TypeCheckServiceStatus, NewCheckServiceStatusJob)
	pkgjobs.RegisterTyped(mux, TypeConfigureOpcache, NewConfigureOpcacheJob)

	// PHP jobs
	pkgjobs.RegisterTyped(mux, TypeSetDefaultPhp, NewSetDefaultPhpJob)
	pkgjobs.RegisterTyped(mux, TypeInstallPhpExtension, NewInstallPhpExtensionJob)
	pkgjobs.RegisterTyped(mux, TypeUninstallPhpExtension, NewUninstallPhpExtensionJob)
	pkgjobs.RegisterTyped(mux, TypeCleanupFailedPhpInstallation, NewCleanupFailedPhpInstallationJob)
	pkgjobs.RegisterTyped(mux, TypeCleanupFailedPhpExtensionInstall, NewCleanupFailedPhpExtensionInstallJob)
	pkgjobs.RegisterTyped(mux, TypeCleanupFailedPhpExtensionUninstall, NewCleanupFailedPhpExtensionUninstallJob)

	// Security audit jobs
	pkgjobs.RegisterTyped(mux, TypeVulnerabilityAudit, NewVulnerabilityAuditJob)

	// Task and maintenance jobs
	pkgjobs.RegisterTyped(mux, TypeCheckDaemonStatus, NewCheckDaemonStatusJob)
	pkgjobs.RegisterTyped(mux, TypeUpdateTaskOutput, NewUpdateTaskOutputJob)
	pkgjobs.RegisterTyped(mux, TypeFetchTaskOutput, NewFetchTaskOutputJob)
	pkgjobs.RegisterTyped(mux, TypeUpdateUserPublicKey, NewUpdateUserPublicKeyJob)
	pkgjobs.RegisterTyped(mux, TypeInstallTaskCleanupCron, NewInstallTaskCleanupCronJob)
	pkgjobs.RegisterTyped(mux, TypeRunAfterUpdate, NewRunAfterUpdateJob)

	// Scheduled maintenance jobs
	pkgjobs.RegisterTyped(mux, TypeCleanupOldMetrics, NewCleanupOldMetricsJob)
	pkgjobs.RegisterTyped(mux, TypeSyncAllDaemons, NewSyncAllDaemonsJob)
	pkgjobs.RegisterTyped(mux, TypeCheckAllConnectivity, NewCheckAllConnectivityJob)

	// Load Balancer jobs
	pkgjobs.RegisterTyped(mux, TypeInstallLBCaddyfile, NewInstallLBCaddyfileJob)
	pkgjobs.RegisterTyped(mux, TypeUpdateLBCaddyfile, NewUpdateLBCaddyfileJob)
	pkgjobs.RegisterTyped(mux, TypeRemoveLBCaddyfile, NewRemoveLBCaddyfileJob)
	pkgjobs.RegisterTyped(mux, TypeAddLBFirewallRule, NewAddLBFirewallRuleJob)
	pkgjobs.RegisterTyped(mux, TypeRemoveLBFirewallRule, NewRemoveLBFirewallRuleJob)
	pkgjobs.RegisterTyped(mux, TypeCheckLBBackendHealth, NewCheckLBBackendHealthJob)
}
