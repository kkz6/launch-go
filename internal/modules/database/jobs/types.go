package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// Job type constants
const (
	TypeInstallDatabase       = "database:install"
	TypeUninstallDatabase     = "database:uninstall"
	TypeInstallDatabaseUser   = "database:user:install"
	TypeUpdateDatabaseUser    = "database:user:update"
	TypeUninstallDatabaseUser = "database:user:uninstall"
	TypeSyncDatabases         = "database:sync"
)

// ===============================
// Payload Types
// ===============================

// InstallDatabasePayload contains data for installing a database.
type InstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

// UninstallDatabasePayload contains data for uninstalling a database.
type UninstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

// InstallDatabaseUserPayload contains data for installing a database user.
type InstallDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	Password       string  `json:"password"`
	CallerID       *string `json:"caller_id,omitempty"`
}

// UpdateDatabaseUserPayload contains data for updating a database user.
type UpdateDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	Password       *string `json:"password,omitempty"`
	CallerID       *string `json:"caller_id,omitempty"`
}

// UninstallDatabaseUserPayload contains data for uninstalling a database user.
type UninstallDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	CallerID       *string `json:"caller_id,omitempty"`
}

// SyncDatabasesPayload contains data for syncing databases on a server.
type SyncDatabasesPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// ===============================
// Task Creation Helpers
// ===============================

// NewInstallDatabaseTask creates a new asynq task for installing a database.
func NewInstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallDatabase, InstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	})
}

// NewUninstallDatabaseTask creates a new asynq task for uninstalling a database.
func NewUninstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallDatabase, UninstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	})
}

// NewInstallDatabaseUserTask creates a new asynq task for installing a database user.
func NewInstallDatabaseUserTask(databaseUserID, password string, callerID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallDatabaseUser, InstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
	})
}

// NewUpdateDatabaseUserTask creates a new asynq task for updating a database user.
func NewUpdateDatabaseUserTask(databaseUserID string, password, callerID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUpdateDatabaseUser, UpdateDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
	})
}

// NewUninstallDatabaseUserTask creates a new asynq task for uninstalling a database user.
func NewUninstallDatabaseUserTask(databaseUserID string, callerID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallDatabaseUser, UninstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		CallerID:       callerID,
	})
}

// NewSyncDatabasesTask creates a new asynq task for syncing databases on a server.
func NewSyncDatabasesTask(serverID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeSyncDatabases, SyncDatabasesPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}
