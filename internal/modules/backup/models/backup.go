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
	ID                        string          `gorm:"primaryKey;size:26" json:"id"`
	ServerID                  string          `gorm:"size:26;not null;index" json:"server_id"`
	UserID                    string          `gorm:"size:26;not null;index" json:"user_id"`
	StorageProviderID         string          `gorm:"size:26;not null;index" json:"storage_provider_id"`
	CronExpression            string          `gorm:"size:100;not null" json:"cron_expression"`
	IncludeFiles              JSON            `gorm:"type:json" json:"include_files"`
	ExcludeFiles              JSON            `gorm:"type:json" json:"exclude_files"`
	Retention                 int             `gorm:"default:10" json:"retention"`
	NotificationOnFailure     bool            `gorm:"default:false" json:"notification_on_failure"`
	NotificationOnSuccess     bool            `gorm:"default:false" json:"notification_on_success"`
	Enabled                   bool            `gorm:"default:true" json:"enabled"`
	Path                      string          `gorm:"size:500" json:"path"`
	DispatchToken             string          `gorm:"size:64;not null" json:"dispatch_token"`
	InstalledAt               *time.Time      `json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time      `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time      `json:"uninstallation_requested_at,omitempty"`
	UninstallationFailedAt    *time.Time      `json:"uninstallation_failed_at,omitempty"`
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
	DeletedAt                 gorm.DeletedAt  `gorm:"index" json:"-"`

	// Relations
	Jobs            []BackupJob      `gorm:"foreignKey:BackupID" json:"jobs,omitempty"`
	StorageProvider *StorageProvider `gorm:"foreignKey:StorageProviderID" json:"storage_provider,omitempty"`
	Databases       []BackupDatabase `gorm:"foreignKey:BackupID" json:"databases,omitempty"`
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
		totalSize += job.Size
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
