package jobs

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/websocket"
)

// Registry holds all job handlers for the server module
type Registry struct {
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
func NewRegistry(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *Registry {
	return &Registry{
		// Server management
		ProvisionServer: NewProvisionServerJob(db, ws, logger),
		RebootServer:    NewRebootServerJob(db, ws, logger),
		DeleteServer:    NewDeleteServerJob(db, ws, logger),

		// PHP version management
		AddPhpVersion:    NewAddPhpVersionJob(db, ws, logger),
		RemovePhpVersion: NewRemovePhpVersionJob(db, ws, logger),

		// SSH key management
		AddSshKey:    NewAddSshKeyJob(db, ws, logger),
		RemoveSshKey: NewRemoveSshKeyJob(db, ws, logger),

		// Firewall management
		InstallFirewallRule:   NewInstallFirewallRuleJob(db, ws, logger),
		UninstallFirewallRule: NewUninstallFirewallRuleJob(db, ws, logger),

		// Daemon management
		InstallDaemon:   NewInstallDaemonJob(db, ws, logger),
		UninstallDaemon: NewUninstallDaemonJob(db, ws, logger),
		RestartDaemon:   NewRestartDaemonJob(db, ws, logger),

		// Cron management
		InstallCron:   NewInstallCronJob(db, ws, logger),
		UninstallCron: NewUninstallCronJob(db, ws, logger),

		// Database management
		InstallDatabase:     NewInstallDatabaseJob(db, ws, logger),
		InstallDatabaseUser: NewInstallDatabaseUserJob(db, ws, logger),

		// Service management
		RestartService:   NewRestartServiceJob(db, ws, logger),
		AddService:       NewAddServiceJob(db, ws, logger),
		RemoveService:    NewRemoveServiceJob(db, ws, logger),
		ServiceOperation: NewServiceOperationJob(db, ws, logger),
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
