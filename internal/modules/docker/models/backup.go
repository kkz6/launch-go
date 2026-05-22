package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// DatabaseBackup is the backup configuration attached to a managed
// database. One row per database (enforced by the migration's unique
// index on database_id). Credentials are stored as encrypted JSON so a
// leaked DB dump doesn't reveal the S3 access key directly.
type DatabaseBackup struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.SoftDeleteModel

	DatabaseID   string                  `gorm:"column:database_id;type:char(26);not null;index" json:"database_id"`
	Provider     string                  `gorm:"type:varchar(32);not null;default:s3" json:"provider"`
	Endpoint     *string                 `gorm:"type:varchar(255)" json:"endpoint,omitempty"`
	Bucket       string                  `gorm:"type:varchar(255);not null" json:"bucket"`
	Region       *string                 `gorm:"type:varchar(64)" json:"region,omitempty"`
	PathPrefix   *string                 `gorm:"column:path_prefix;type:varchar(255)" json:"path_prefix,omitempty"`
	Credentials  dbtype.EncryptedString  `gorm:"type:longtext" json:"-"`
	CronSchedule *string                 `gorm:"column:cron_schedule;type:varchar(64)" json:"cron_schedule,omitempty"`
	Enabled      bool                    `gorm:"not null;default:true" json:"enabled"`
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
