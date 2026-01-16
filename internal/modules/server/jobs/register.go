package jobs

import (
	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var jobContext *JobContext

// SetJobContext sets the global job context.
func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

// RegisterHandlers registers all server job handlers with the asynq mux.
func RegisterHandlers(mux *asynq.ServeMux) {
	// Server lifecycle jobs
	pkgjobs.RegisterHandler(mux, TypeCreateOnProvider, jobContext, NewCreateOnProviderJob)
	pkgjobs.RegisterHandler(mux, TypeProvisionServer, jobContext, NewProvisionServerJob)
	pkgjobs.RegisterHandler(mux, TypeDeleteServer, jobContext, NewDeleteServerJob)
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
	pkgjobs.RegisterHandler(mux, TypeAddSshKey, jobContext, NewAddSshKeyJob)
	pkgjobs.RegisterHandler(mux, TypeRemoveSshKey, jobContext, NewRemoveSshKeyJob)

	// Service jobs
	pkgjobs.RegisterHandler(mux, TypeAddService, jobContext, NewAddServiceJob)
	pkgjobs.RegisterHandler(mux, TypeRemoveService, jobContext, NewRemoveServiceJob)
	pkgjobs.RegisterHandler(mux, TypeServiceOperation, jobContext, NewServiceOperationJob)
	pkgjobs.RegisterHandler(mux, TypeCheckServiceStatus, jobContext, NewCheckServiceStatusJob)
	pkgjobs.RegisterHandler(mux, TypeConfigureOpcache, jobContext, NewConfigureOpcacheJob)

	// Security audit jobs
	pkgjobs.RegisterHandler(mux, TypeVulnerabilityAudit, jobContext, NewVulnerabilityAuditJob)
}
