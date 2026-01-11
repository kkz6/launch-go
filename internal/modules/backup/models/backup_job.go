package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// BackupJob represents an individual backup execution
type BackupJob struct {
	ID                string               `gorm:"primaryKey;size:26" json:"id"`
	BackupID          string               `gorm:"size:26;not null;index" json:"backup_id"`
	StorageProviderID string               `gorm:"size:26;not null;index" json:"storage_provider_id"`
	Status            enums.BackupJobStatus `gorm:"size:20;not null;default:'pending'" json:"status"`
	Size              int64                `gorm:"default:0" json:"size"`
	Error             *string              `gorm:"type:text" json:"error,omitempty"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`

	// Relations
	Backup          *Backup          `gorm:"foreignKey:BackupID" json:"backup,omitempty"`
	StorageProvider *StorageProvider `gorm:"foreignKey:StorageProviderID" json:"storage_provider,omitempty"`
}

// BeforeCreate hook generates ULID
func (j *BackupJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == "" {
		j.ID = utils.NewULID()
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
	return j.Size / 1024 / 1024
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
