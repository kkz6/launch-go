package jobs

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/websocket"
)

// Registry holds all job handlers for the server module
type Registry struct {
	ProvisionServer     *ProvisionServerJob
	AddPhpVersion       *AddPhpVersionJob
	RemovePhpVersion    *RemovePhpVersionJob
	AddSshKey           *AddSshKeyJob
	RemoveSshKey        *RemoveSshKeyJob
	InstallFirewallRule *InstallFirewallRuleJob
	InstallDaemon       *InstallDaemonJob
	RestartDaemon       *RestartDaemonJob
	InstallCron         *InstallCronJob
	InstallDatabase     *InstallDatabaseJob
	InstallDatabaseUser *InstallDatabaseUserJob
	RestartService      *RestartServiceJob
	AddService          *AddServiceJob
	RemoveService       *RemoveServiceJob
}

// NewRegistry creates a new registry with all job handlers initialized
func NewRegistry(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *Registry {
	return &Registry{
		ProvisionServer:     NewProvisionServerJob(db, ws, logger),
		AddPhpVersion:       NewAddPhpVersionJob(db, ws, logger),
		RemovePhpVersion:    NewRemovePhpVersionJob(db, ws, logger),
		AddSshKey:           NewAddSshKeyJob(db, ws, logger),
		RemoveSshKey:        NewRemoveSshKeyJob(db, ws, logger),
		InstallFirewallRule: NewInstallFirewallRuleJob(db, ws, logger),
		InstallDaemon:       NewInstallDaemonJob(db, ws, logger),
		RestartDaemon:       NewRestartDaemonJob(db, ws, logger),
		InstallCron:         NewInstallCronJob(db, ws, logger),
		InstallDatabase:     NewInstallDatabaseJob(db, ws, logger),
		InstallDatabaseUser: NewInstallDatabaseUserJob(db, ws, logger),
		RestartService:      NewRestartServiceJob(db, ws, logger),
		AddService:          NewAddServiceJob(db, ws, logger),
		RemoveService:       NewRemoveServiceJob(db, ws, logger),
	}
}

// RegisterHandlers registers all server job handlers with the asynq ServeMux
func (r *Registry) RegisterHandlers(mux *asynq.ServeMux) {
	// Server provisioning
	mux.HandleFunc(TypeProvisionServer, r.ProvisionServer.Handle)

	// PHP version management
	mux.HandleFunc(TypeAddPhpVersion, r.AddPhpVersion.Handle)
	mux.HandleFunc(TypeRemovePhpVersion, r.RemovePhpVersion.Handle)

	// SSH key management
	mux.HandleFunc(TypeAddSshKey, r.AddSshKey.Handle)
	mux.HandleFunc(TypeRemoveSshKey, r.RemoveSshKey.Handle)

	// Firewall management
	mux.HandleFunc(TypeInstallFirewallRule, r.InstallFirewallRule.Handle)

	// Daemon management
	mux.HandleFunc(TypeInstallDaemon, r.InstallDaemon.Handle)
	mux.HandleFunc(TypeRestartDaemon, r.RestartDaemon.Handle)

	// Cron management
	mux.HandleFunc(TypeInstallCron, r.InstallCron.Handle)

	// Database management
	mux.HandleFunc(TypeInstallDatabase, r.InstallDatabase.Handle)
	mux.HandleFunc(TypeInstallDatabaseUser, r.InstallDatabaseUser.Handle)

	// Service management
	mux.HandleFunc(TypeRestartService, r.RestartService.Handle)
	mux.HandleFunc(TypeAddService, r.AddService.Handle)
	mux.HandleFunc(TypeRemoveService, r.RemoveService.Handle)
}

// AllTaskTypes returns all task type constants for this module
func AllTaskTypes() []string {
	return []string{
		TypeProvisionServer,
		TypeAddPhpVersion,
		TypeRemovePhpVersion,
		TypeAddSshKey,
		TypeRemoveSshKey,
		TypeInstallFirewallRule,
		TypeInstallDaemon,
		TypeRestartDaemon,
		TypeInstallCron,
		TypeInstallDatabase,
		TypeInstallDatabaseUser,
		TypeRestartService,
		TypeAddService,
		TypeRemoveService,
	}
}
