package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const (
	TypeInstallDatabase       = "database:install"
	TypeUninstallDatabase     = "database:uninstall"
	TypeInstallDatabaseUser   = "database:user:install"
	TypeUpdateDatabaseUser    = "database:user:update"
	TypeUninstallDatabaseUser = "database:user:uninstall"
	TypeSyncDatabases         = "database:sync"
)

type InstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

type UninstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

type InstallDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	Password       string  `json:"password"`
	CallerID       *string `json:"caller_id,omitempty"`
}

type UpdateDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	Password       *string `json:"password,omitempty"`
	CallerID       *string `json:"caller_id,omitempty"`
}

type UninstallDatabaseUserPayload struct {
	DatabaseUserID string  `json:"database_user_id"`
	CallerID       *string `json:"caller_id,omitempty"`
}

type SyncDatabasesPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

func NewInstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallDatabase, InstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	})
}

func NewUninstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallDatabase, UninstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	})
}

func NewInstallDatabaseUserTask(databaseUserID, password string, callerID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallDatabaseUser, InstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
	})
}

func NewUpdateDatabaseUserTask(databaseUserID string, password, callerID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUpdateDatabaseUser, UpdateDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		Password:       password,
		CallerID:       callerID,
	})
}

func NewUninstallDatabaseUserTask(databaseUserID string, callerID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallDatabaseUser, UninstallDatabaseUserPayload{
		DatabaseUserID: databaseUserID,
		CallerID:       callerID,
	})
}

func NewSyncDatabasesTask(serverID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeSyncDatabases, SyncDatabasesPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}
