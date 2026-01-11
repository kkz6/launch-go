package jobs

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/websocket"
)

// Registry holds all job handlers for the database module
type Registry struct {
	InstallDatabase       *InstallDatabaseJob
	UninstallDatabase     *UninstallDatabaseJob
	InstallDatabaseUser   *InstallDatabaseUserJob
	UpdateDatabaseUser    *UpdateDatabaseUserJob
	UninstallDatabaseUser *UninstallDatabaseUserJob
	SyncDatabases         *SyncDatabasesJob
}

// NewRegistry creates a new registry with all job handlers initialized
func NewRegistry(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *Registry {
	return &Registry{
		InstallDatabase:       NewInstallDatabaseJob(db, ws, logger),
		UninstallDatabase:     NewUninstallDatabaseJob(db, ws, logger),
		InstallDatabaseUser:   NewInstallDatabaseUserJob(db, ws, logger),
		UpdateDatabaseUser:    NewUpdateDatabaseUserJob(db, ws, logger),
		UninstallDatabaseUser: NewUninstallDatabaseUserJob(db, ws, logger),
		SyncDatabases:         NewSyncDatabasesJob(db, ws, logger),
	}
}

// RegisterHandlers registers all database job handlers with the asynq ServeMux
func (r *Registry) RegisterHandlers(mux *asynq.ServeMux) {
	// Database management
	mux.HandleFunc(TypeInstallDatabase, r.InstallDatabase.Handle)
	mux.HandleFunc(TypeUninstallDatabase, r.UninstallDatabase.Handle)

	// Database user management
	mux.HandleFunc(TypeInstallDatabaseUser, r.InstallDatabaseUser.Handle)
	mux.HandleFunc(TypeUpdateDatabaseUser, r.UpdateDatabaseUser.Handle)
	mux.HandleFunc(TypeUninstallDatabaseUser, r.UninstallDatabaseUser.Handle)

	// Sync operations
	mux.HandleFunc(TypeSyncDatabases, r.SyncDatabases.Handle)
}

// AllTaskTypes returns all task type constants for this module
func AllTaskTypes() []string {
	return []string{
		TypeInstallDatabase,
		TypeUninstallDatabase,
		TypeInstallDatabaseUser,
		TypeUpdateDatabaseUser,
		TypeUninstallDatabaseUser,
		TypeSyncDatabases,
	}
}
