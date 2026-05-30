package models

import (
	"time"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// DatabaseBackup is the backup configuration attached to a managed
// database. One row per database (enforced by the migration's unique
// index on database_id).
//
// Credentials are NOT stored on this row — instead the row references
// a global storage_providers entry via StorageProviderID. The same
// pattern the rest of the platform uses (server backups, site
// snapshots) so the user configures a destination once under
// Settings → Connections and reuses it everywhere.
type DatabaseBackup struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.SoftDeleteModel

	DatabaseID        string `gorm:"column:database_id;type:char(26);not null;index" json:"database_id"`
	StorageProviderID uint64 `gorm:"column:storage_provider_id;not null;index" json:"storage_provider_id"`
	// DatabaseName optionally overrides which database INSIDE the engine
	// the dump command targets. Empty / nil = use the database
	// provisioned with this row (today's behaviour). Set when the user
	// has created additional databases inside the engine (e.g.
	// `CREATE DATABASE analytics;` on the same Postgres container) and
	// wants this backup config to target one of those instead.
	//
	// Stored on the config row rather than on each run because the
	// scheduler needs to know the target without consulting history.
	DatabaseName *string `gorm:"column:database_name;type:varchar(64)" json:"database_name,omitempty"`
	// Path is the sub-folder under the storage provider's bucket where
	// this database's dumps land. Optional; empty = bucket root.
	Path *string `gorm:"type:varchar(255)" json:"path,omitempty"`
	// Retention caps the number of historical run rows we keep. Older
	// rows + their remote objects are pruned after each successful run.
	Retention int `gorm:"not null;default:10" json:"retention"`
	// NotifyOnSuccess / NotifyOnFailure feed the notifications module
	// — drives whether a success / failure email or webhook fires.
	NotifyOnSuccess bool    `gorm:"column:notify_on_success;not null;default:false" json:"notify_on_success"`
	NotifyOnFailure bool    `gorm:"column:notify_on_failure;not null;default:true" json:"notify_on_failure"`
	CronSchedule    *string `gorm:"column:cron_schedule;type:varchar(64)" json:"cron_schedule,omitempty"`
	Enabled         bool    `gorm:"not null;default:true" json:"enabled"`
}

func (DatabaseBackup) TableName() string { return "docker_database_backups" }

// EffectiveDatabaseName returns the database the dump command should
// target: the per-backup override if the user set one, else the
// fallback (which the caller pulls from the docker_databases row's
// stored credentials). Centralised here so the synchronous RunNow
// path (services/backup_service.go) and the scheduled cron path
// (jobs/run_backup.go) can't drift — a divergence would mean manual
// and scheduled runs of the same config target different databases.
func (b *DatabaseBackup) EffectiveDatabaseName(fallback string) string {
	if b == nil || b.DatabaseName == nil {
		return fallback
	}
	if *b.DatabaseName == "" {
		return fallback
	}
	return *b.DatabaseName
}

// DatabaseBackupRun is one past upload attempt — what the UI's history
// table renders, and what restore-from-snapshot identifies.
type DatabaseBackupRun struct {
	basemodels.BaseModel

	BackupID   string     `gorm:"column:backup_id;type:char(26);not null;index" json:"backup_id"`
	Status     string     `gorm:"type:varchar(32);not null;default:running" json:"status"`
	StartedAt  *time.Time `gorm:"column:started_at;type:timestamp null" json:"started_at,omitempty"`
	FinishedAt *time.Time `gorm:"column:finished_at;type:timestamp null" json:"finished_at,omitempty"`
	ObjectKey  *string    `gorm:"column:object_key;type:varchar(512)" json:"object_key,omitempty"`
	SizeBytes  *int64     `gorm:"column:size_bytes" json:"size_bytes,omitempty"`
	Error      *string    `gorm:"type:text" json:"error,omitempty"`
	// TaskID links to the server-tasks row the worker created for this
	// run; the UI uses it to stream the live dump/upload output
	// (ServerLogViewer entity="task"). NULL for pre-task-tracking runs.
	TaskID *string `gorm:"column:task_id;type:char(26);index" json:"task_id,omitempty"`
}

func (DatabaseBackupRun) TableName() string { return "docker_database_backup_runs" }
