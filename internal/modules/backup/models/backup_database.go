package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// BackupDatabase represents the many-to-many relationship between backups and databases
type BackupDatabase struct {
	ID         string    `gorm:"primaryKey;size:26" json:"id"`
	BackupID   string    `gorm:"size:26;not null;index" json:"backup_id"`
	DatabaseID string    `gorm:"size:26;not null;index" json:"database_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// BeforeCreate hook generates ULID
func (bd *BackupDatabase) BeforeCreate(tx *gorm.DB) error {
	if bd.ID == "" {
		bd.ID = utils.NewULID()
	}

	return nil
}

// TableName returns the table name for BackupDatabase
func (BackupDatabase) TableName() string {
	return "backup_databases"
}
