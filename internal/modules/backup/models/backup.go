package models

import (
	"gorm.io/gorm"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/token"
)

// Backup represents a backup configuration for a server
type Backup struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	basemodels.ServerScopedModel
	basemodels.TeamScopedModel
	UserID                *string `gorm:"column:user_id;type:char(26);index" json:"user_id,omitempty"`
	StorageProviderID     uint64  `gorm:"column:storage_provider_id;not null;index" json:"storage_provider_id"`
	DispatchToken         string  `gorm:"column:dispatch_token;type:varchar(32);not null" json:"dispatch_token"`
	CronExpression        string  `gorm:"column:cron_expression;type:varchar(255);not null" json:"cron_expression"`
	IncludeFiles          string  `gorm:"column:include_files;type:json;not null" json:"include_files"`
	ExcludeFiles          string  `gorm:"column:exclude_files;type:json;not null" json:"exclude_files"`
	Retention             int     `gorm:"default:14" json:"retention"`
	NotificationOnFailure bool    `gorm:"column:notification_on_failure;default:true" json:"notification_on_failure"`
	NotificationOnSuccess bool    `gorm:"column:notification_on_success;default:true" json:"notification_on_success"`
	Enabled               bool    `gorm:"default:false" json:"enabled"`
	Path                  string  `gorm:"type:varchar(255);not null" json:"path"`

	// Relations
	Jobs            []BackupJob      `gorm:"foreignKey:BackupID;references:ID" json:"jobs,omitempty"`
	StorageProvider *StorageProvider `gorm:"foreignKey:StorageProviderID;references:ID" json:"storage_provider,omitempty"`
	Databases       []BackupDatabase `gorm:"foreignKey:BackupID;references:ID" json:"databases,omitempty"`
}

// BeforeCreate hook generates ULID and dispatch token
func (b *Backup) BeforeCreate(tx *gorm.DB) error {
	if err := b.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	if b.DispatchToken == "" {
		b.DispatchToken = token.New(32).WithEncoding(token.Base64URL).MustGenerate()
	}

	if b.Retention == 0 {
		b.Retention = 10
	}

	return nil
}

// TableName returns the table name for Backup
func (Backup) TableName() string {
	return "backups"
}

// GetSizeInMB calculates total backup size in megabytes from all jobs
func (b *Backup) GetSizeInMB() int64 {
	var totalSize int64
	for _, job := range b.Jobs {
		if job.Size != nil {
			totalSize += int64(*job.Size)
		}
	}

	return totalSize / 1024 / 1024
}
