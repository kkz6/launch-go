package jobs

import (
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

var jobContext *JobContext

// SetJobContext sets the global job context.
func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

func getContext() any {
	return jobContext
}

// Register registers all server jobs with the registry.
func Register(r *jobs.Registry) {
	// Server lifecycle jobs
	r.Register(TypeCreateOnProvider, jobs.MakeFactory(
		func(p CreateOnProviderPayload) *CreateOnProviderJob { return &CreateOnProviderJob{Payload: p} },
		getContext,
	))
	r.Register(TypeProvisionServer, jobs.MakeFactory(
		func(p ProvisionServerPayload) *ProvisionServerJob { return &ProvisionServerJob{Payload: p} },
		getContext,
	))
	r.Register(TypeDeleteServer, jobs.MakeFactory(
		func(p DeleteServerPayload) *DeleteServerJob { return &DeleteServerJob{Payload: p} },
		getContext,
	))
	r.Register(TypeRebootServer, jobs.MakeFactory(
		func(p RebootServerPayload) *RebootServerJob { return &RebootServerJob{Payload: p} },
		getContext,
	))
	r.Register(TypeArchiveServer, jobs.MakeFactory(
		func(p ArchiveServerPayload) *ArchiveServerJob { return &ArchiveServerJob{Payload: p} },
		getContext,
	))
	r.Register(TypeUnarchiveServer, jobs.MakeFactory(
		func(p UnarchiveServerPayload) *UnarchiveServerJob { return &UnarchiveServerJob{Payload: p} },
		getContext,
	))
	r.Register(TypeUpdateConnectivity, jobs.MakeFactory(
		func(p UpdateConnectivityPayload) *UpdateConnectivityJob { return &UpdateConnectivityJob{Payload: p} },
		getContext,
	))

	// Cron jobs
	r.Register(TypeInstallCron, jobs.MakeFactory(
		func(p InstallCronPayload) *InstallCronJob { return &InstallCronJob{Payload: p} },
		getContext,
	))
	r.Register(TypeUninstallCron, jobs.MakeFactory(
		func(p UninstallCronPayload) *UninstallCronJob { return &UninstallCronJob{Payload: p} },
		getContext,
	))

	// Daemon jobs
	r.Register(TypeInstallDaemon, jobs.MakeFactory(
		func(p InstallDaemonPayload) *InstallDaemonJob { return &InstallDaemonJob{Payload: p} },
		getContext,
	))
	r.Register(TypeUninstallDaemon, jobs.MakeFactory(
		func(p UninstallDaemonPayload) *UninstallDaemonJob { return &UninstallDaemonJob{Payload: p} },
		getContext,
	))
	r.Register(TypeRestartDaemon, jobs.MakeFactory(
		func(p RestartDaemonPayload) *RestartDaemonJob { return &RestartDaemonJob{Payload: p} },
		getContext,
	))
	r.Register(TypeSyncDaemons, jobs.MakeFactory(
		func(p SyncDaemonsPayload) *SyncDaemonsJob { return &SyncDaemonsJob{Payload: p} },
		getContext,
	))

	// Firewall jobs
	r.Register(TypeInstallFirewallRule, jobs.MakeFactory(
		func(p InstallFirewallRulePayload) *InstallFirewallRuleJob { return &InstallFirewallRuleJob{Payload: p} },
		getContext,
	))
	r.Register(TypeUninstallFirewall, jobs.MakeFactory(
		func(p UninstallFirewallRulePayload) *UninstallFirewallRuleJob { return &UninstallFirewallRuleJob{Payload: p} },
		getContext,
	))

	// SSH key jobs
	r.Register(TypeAddSshKey, jobs.MakeFactory(
		func(p AddSshKeyPayload) *AddSshKeyJob { return &AddSshKeyJob{Payload: p} },
		getContext,
	))
	r.Register(TypeRemoveSshKey, jobs.MakeFactory(
		func(p RemoveSshKeyPayload) *RemoveSshKeyJob { return &RemoveSshKeyJob{Payload: p} },
		getContext,
	))

	// Service jobs
	r.Register(TypeAddService, jobs.MakeFactory(
		func(p AddServicePayload) *AddServiceJob { return &AddServiceJob{Payload: p} },
		getContext,
	))
	r.Register(TypeRemoveService, jobs.MakeFactory(
		func(p RemoveServicePayload) *RemoveServiceJob { return &RemoveServiceJob{Payload: p} },
		getContext,
	))
	r.Register(TypeServiceOperation, jobs.MakeFactory(
		func(p ServiceOperationPayload) *ServiceOperationJob { return &ServiceOperationJob{Payload: p} },
		getContext,
	))
	r.Register(TypeCheckServiceStatus, jobs.MakeFactory(
		func(p CheckServiceStatusPayload) *CheckServiceStatusJob { return &CheckServiceStatusJob{Payload: p} },
		getContext,
	))
	r.Register(TypeConfigureOpcache, jobs.MakeFactory(
		func(p ConfigureOpcachePayload) *ConfigureOpcacheJob { return &ConfigureOpcacheJob{Payload: p} },
		getContext,
	))

	// Security audit jobs
	r.Register(TypeVulnerabilityAudit, jobs.MakeFactory(
		func(p VulnerabilityAuditPayload) *VulnerabilityAuditJob { return &VulnerabilityAuditJob{Payload: p} },
		getContext,
	))
}
