package models

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Backup represents a backup configuration for a server
type Backup struct {
	ID                        string     `gorm:"type:char(26);primaryKey" json:"id"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	UserID                    *string    `gorm:"column:user_id;type:char(26);index" json:"user_id,omitempty"`
	StorageProviderID         uint64     `gorm:"column:storage_provider_id;not null;index" json:"storage_provider_id"`
	DispatchToken             string     `gorm:"column:dispatch_token;type:varchar(32);not null" json:"dispatch_token"`
	CronExpression            string     `gorm:"column:cron_expression;type:varchar(255);not null" json:"cron_expression"`
	IncludeFiles              string     `gorm:"column:include_files;type:json;not null" json:"include_files"`
	ExcludeFiles              string     `gorm:"column:exclude_files;type:json;not null" json:"exclude_files"`
	Retention                 int        `gorm:"default:14" json:"retention"`
	NotificationOnFailure     bool       `gorm:"column:notification_on_failure;default:true" json:"notification_on_failure"`
	NotificationOnSuccess     bool       `gorm:"column:notification_on_success;default:true" json:"notification_on_success"`
	Enabled                   bool       `gorm:"default:false" json:"enabled"`
	Path                      string     `gorm:"type:varchar(255);not null" json:"path"`
	InstalledAt               *time.Time `gorm:"column:installed_at;type:timestamp null" json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `gorm:"column:installation_failed_at;type:timestamp null" json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null" json:"uninstallation_requested_at,omitempty"`
	UninstallationFailedAt    *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null" json:"uninstallation_failed_at,omitempty"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Jobs            []BackupJob      `gorm:"foreignKey:BackupID;references:ID" json:"jobs,omitempty"`
	StorageProvider *StorageProvider `gorm:"foreignKey:StorageProviderID;references:ID" json:"storage_provider,omitempty"`
	Databases       []BackupDatabase `gorm:"foreignKey:BackupID;references:ID" json:"databases,omitempty"`
}

// BeforeCreate hook generates ULID and dispatch token
func (b *Backup) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = utils.NewULID()
	}

	if b.DispatchToken == "" {
		token := make([]byte, 32)
		if _, err := rand.Read(token); err != nil {
			return err
		}
		b.DispatchToken = base64.URLEncoding.EncodeToString(token)
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

// IsInstalled returns true if the backup is installed
func (b *Backup) IsInstalled() bool {
	return b.InstalledAt != nil && b.InstallationFailedAt == nil
}

// IsPendingInstallation returns true if installation is pending
func (b *Backup) IsPendingInstallation() bool {
	return b.InstalledAt == nil && b.InstallationFailedAt == nil
}

// IsInstallationFailed returns true if installation failed
func (b *Backup) IsInstallationFailed() bool {
	return b.InstallationFailedAt != nil
}
