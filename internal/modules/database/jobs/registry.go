package jobs

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
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
func NewRegistry(db *gorm.DB, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, queueClient *queue.Client, logger *zerolog.Logger) *Registry {
	return &Registry{
		InstallDatabase:       NewInstallDatabaseJob(db, ws, dispatcher, queueClient, logger),
		UninstallDatabase:     NewUninstallDatabaseJob(db, ws, dispatcher, queueClient, logger),
		InstallDatabaseUser:   NewInstallDatabaseUserJob(db, ws, dispatcher, queueClient, logger),
		UpdateDatabaseUser:    NewUpdateDatabaseUserJob(db, ws, dispatcher, queueClient, logger),
		UninstallDatabaseUser: NewUninstallDatabaseUserJob(db, ws, dispatcher, queueClient, logger),
		SyncDatabases:         NewSyncDatabasesJob(db, ws, dispatcher, queueClient, logger),
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
