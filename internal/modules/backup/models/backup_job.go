package models

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/enums"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// BackupJob represents an individual backup execution
type BackupJob struct {
	basemodels.BaseModel
	Status            enums.BackupJobStatus `gorm:"type:varchar(255);not null" json:"status"`
	BackupID          string                `gorm:"column:backup_id;type:char(26);not null;index" json:"backup_id"`
	StorageProviderID uint64                `gorm:"column:storage_provider_id;not null;index" json:"storage_provider_id"`
	Size              *int                  `gorm:"type:int" json:"size,omitempty"`
	Error             *string               `gorm:"type:longtext" json:"error,omitempty"`

	// Relations
	Backup          *Backup          `gorm:"foreignKey:BackupID;references:ID" json:"backup,omitempty"`
	StorageProvider *StorageProvider `gorm:"foreignKey:StorageProviderID;references:ID" json:"storage_provider,omitempty"`
}

// BeforeCreate hook generates ULID
func (j *BackupJob) BeforeCreate(tx *gorm.DB) error {
	if err := j.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	if j.Status == "" {
		j.Status = enums.BackupJobStatusPending
	}

	return nil
}

// TableName returns the table name for BackupJob
func (BackupJob) TableName() string {
	return "backup_jobs"
}

// GetSizeInMB returns the backup size in megabytes
func (j *BackupJob) GetSizeInMB() int64 {
	if j.Size == nil {
		return 0
	}

	return int64(*j.Size) / 1024 / 1024
}

// IsFinished returns true if the backup job has finished successfully
func (j *BackupJob) IsFinished() bool {
	return j.Status == enums.BackupJobStatusFinished
}

// IsFailed returns true if the backup job has failed
func (j *BackupJob) IsFailed() bool {
	return j.Status == enums.BackupJobStatusFailed
}

// IsRunning returns true if the backup job is currently running
func (j *BackupJob) IsRunning() bool {
	return j.Status == enums.BackupJobStatusRunning
}

// IsPending returns true if the backup job is pending
func (j *BackupJob) IsPending() bool {
	return j.Status == enums.BackupJobStatusPending
}
