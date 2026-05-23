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

	DatabaseID        string  `gorm:"column:database_id;type:char(26);not null;index" json:"database_id"`
	StorageProviderID uint64  `gorm:"column:storage_provider_id;not null;index" json:"storage_provider_id"`
	// Path is the sub-folder under the storage provider's bucket where
	// this database's dumps land. Optional; empty = bucket root.
	Path *string `gorm:"type:varchar(255)" json:"path,omitempty"`
	// Retention caps the number of historical run rows we keep. Older
	// rows + their remote objects are pruned after each successful run.
	Retention int `gorm:"not null;default:10" json:"retention"`
	// NotifyOnSuccess / NotifyOnFailure feed the notifications module
	// — drives whether a success / failure email or webhook fires.
	NotifyOnSuccess bool `gorm:"column:notify_on_success;not null;default:false" json:"notify_on_success"`
	NotifyOnFailure bool `gorm:"column:notify_on_failure;not null;default:true" json:"notify_on_failure"`
	CronSchedule    *string `gorm:"column:cron_schedule;type:varchar(64)" json:"cron_schedule,omitempty"`
	Enabled         bool    `gorm:"not null;default:true" json:"enabled"`
}

func (DatabaseBackup) TableName() string { return "docker_database_backups" }

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
}

func (DatabaseBackupRun) TableName() string { return "docker_database_backup_runs" }
