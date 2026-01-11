package jobs

import (
	"github.com/hibiken/asynq"
)

// Registry holds all job handlers for the server module
type Registry struct {
	ctx *JobContext

	// Server management
	ProvisionServer *ProvisionServerJob
	RebootServer    *RebootServerJob
	DeleteServer    *DeleteServerJob

	// PHP version management
	AddPhpVersion    *AddPhpVersionJob
	RemovePhpVersion *RemovePhpVersionJob

	// SSH key management
	AddSshKey    *AddSshKeyJob
	RemoveSshKey *RemoveSshKeyJob

	// Firewall management
	InstallFirewallRule   *InstallFirewallRuleJob
	UninstallFirewallRule *UninstallFirewallRuleJob

	// Daemon management
	InstallDaemon   *InstallDaemonJob
	UninstallDaemon *UninstallDaemonJob
	RestartDaemon   *RestartDaemonJob

	// Cron management
	InstallCron   *InstallCronJob
	UninstallCron *UninstallCronJob

	// Database management
	InstallDatabase     *InstallDatabaseJob
	InstallDatabaseUser *InstallDatabaseUserJob

	// Service management
	RestartService   *RestartServiceJob
	AddService       *AddServiceJob
	RemoveService    *RemoveServiceJob
	ServiceOperation *ServiceOperationJob
}

// NewRegistry creates a new registry with all job handlers initialized
func NewRegistry(ctx *JobContext) *Registry {
	return &Registry{
		ctx: ctx,

		// Server management
		ProvisionServer: &ProvisionServerJob{JobContext: ctx},
		RebootServer:    &RebootServerJob{JobContext: ctx},
		DeleteServer:    &DeleteServerJob{JobContext: ctx},

		// PHP version management
		AddPhpVersion:    &AddPhpVersionJob{JobContext: ctx},
		RemovePhpVersion: &RemovePhpVersionJob{JobContext: ctx},

		// SSH key management
		AddSshKey:    &AddSshKeyJob{JobContext: ctx},
		RemoveSshKey: &RemoveSshKeyJob{JobContext: ctx},

		// Firewall management
		InstallFirewallRule:   &InstallFirewallRuleJob{JobContext: ctx},
		UninstallFirewallRule: &UninstallFirewallRuleJob{JobContext: ctx},

		// Daemon management
		InstallDaemon:   &InstallDaemonJob{JobContext: ctx},
		UninstallDaemon: &UninstallDaemonJob{JobContext: ctx},
		RestartDaemon:   &RestartDaemonJob{JobContext: ctx},

		// Cron management
		InstallCron:   &InstallCronJob{JobContext: ctx},
		UninstallCron: &UninstallCronJob{JobContext: ctx},

		// Database management
		InstallDatabase:     &InstallDatabaseJob{JobContext: ctx},
		InstallDatabaseUser: &InstallDatabaseUserJob{JobContext: ctx},

		// Service management
		RestartService:   &RestartServiceJob{JobContext: ctx},
		AddService:       &AddServiceJob{JobContext: ctx},
		RemoveService:    &RemoveServiceJob{JobContext: ctx},
		ServiceOperation: &ServiceOperationJob{JobContext: ctx},
	}
}

// RegisterHandlers registers all server job handlers with the asynq ServeMux
func (r *Registry) RegisterHandlers(mux *asynq.ServeMux) {
	// Server management
	mux.HandleFunc(TypeProvisionServer, r.ProvisionServer.Handle)
	mux.HandleFunc(TypeRebootServer, r.RebootServer.Handle)
	mux.HandleFunc(TypeDeleteServer, r.DeleteServer.Handle)

	// PHP version management
	mux.HandleFunc(TypeAddPhpVersion, r.AddPhpVersion.Handle)
	mux.HandleFunc(TypeRemovePhpVersion, r.RemovePhpVersion.Handle)

	// SSH key management
	mux.HandleFunc(TypeAddSshKey, r.AddSshKey.Handle)
	mux.HandleFunc(TypeRemoveSshKey, r.RemoveSshKey.Handle)

	// Firewall management
	mux.HandleFunc(TypeInstallFirewallRule, r.InstallFirewallRule.Handle)
	mux.HandleFunc(TypeUninstallFirewallRule, r.UninstallFirewallRule.Handle)

	// Daemon management
	mux.HandleFunc(TypeInstallDaemon, r.InstallDaemon.Handle)
	mux.HandleFunc(TypeUninstallDaemon, r.UninstallDaemon.Handle)
	mux.HandleFunc(TypeRestartDaemon, r.RestartDaemon.Handle)

	// Cron management
	mux.HandleFunc(TypeInstallCron, r.InstallCron.Handle)
	mux.HandleFunc(TypeUninstallCron, r.UninstallCron.Handle)

	// Database management
	mux.HandleFunc(TypeInstallDatabase, r.InstallDatabase.Handle)
	mux.HandleFunc(TypeInstallDatabaseUser, r.InstallDatabaseUser.Handle)

	// Service management
	mux.HandleFunc(TypeRestartService, r.RestartService.Handle)
	mux.HandleFunc(TypeAddService, r.AddService.Handle)
	mux.HandleFunc(TypeRemoveService, r.RemoveService.Handle)
	mux.HandleFunc(TypeServiceOperation, r.ServiceOperation.Handle)
}

// AllTaskTypes returns all task type constants for this module
func AllTaskTypes() []string {
	return []string{
		TypeProvisionServer,
		TypeRebootServer,
		TypeDeleteServer,
		TypeAddPhpVersion,
		TypeRemovePhpVersion,
		TypeAddSshKey,
		TypeRemoveSshKey,
		TypeInstallFirewallRule,
		TypeUninstallFirewallRule,
		TypeInstallDaemon,
		TypeUninstallDaemon,
		TypeRestartDaemon,
		TypeInstallCron,
		TypeUninstallCron,
		TypeInstallDatabase,
		TypeInstallDatabaseUser,
		TypeRestartService,
		TypeAddService,
		TypeRemoveService,
		TypeServiceOperation,
	}
}
