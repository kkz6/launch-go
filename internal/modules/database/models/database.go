package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Compile-time check that Database uses gorm.DB in BeforeCreate
var _ = (*gorm.DB)(nil)

// Database represents a database on a server
type Database struct {
	ID                        string     `gorm:"type:char(26);primaryKey" json:"id"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	Name                      string     `gorm:"type:varchar(255);not null" json:"name"`
	InstalledAt               *time.Time `gorm:"column:installed_at;type:timestamp null" json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `gorm:"column:installation_failed_at;type:timestamp null" json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null" json:"uninstallation_requested_at,omitempty"`
	UninstallationFailedAt    *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null" json:"uninstallation_failed_at,omitempty"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Users []DatabaseUser `gorm:"many2many:database_database_user;" json:"users,omitempty"`
}

func (Database) TableName() string {
	return "databases"
}

func (d *Database) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = utils.NewULID()
	}

	return nil
}

// Status returns the current installation status
func (d *Database) Status() enums.InstallationStatus {
	if d.UninstallationRequestedAt != nil && d.UninstallationFailedAt == nil {
		return enums.StatusUninstalling
	}

	if d.InstallationFailedAt != nil {
		return enums.StatusFailed
	}

	if d.InstalledAt != nil {
		return enums.StatusInstalled
	}

	return enums.StatusPending
}

// IsInstalled returns true if the database is installed
func (d *Database) IsInstalled() bool {
	return d.InstalledAt != nil && d.InstallationFailedAt == nil
}

// IsInstalling returns true if the database is being installed
func (d *Database) IsInstalling() bool {
	return d.InstalledAt == nil && d.InstallationFailedAt == nil
}

// IsFailed returns true if the installation failed
func (d *Database) IsFailed() bool {
	return d.InstallationFailedAt != nil
}

// IsUninstalling returns true if the database is being uninstalled
func (d *Database) IsUninstalling() bool {
	return d.UninstallationRequestedAt != nil && d.UninstallationFailedAt == nil
}

// MarkAsInstalled marks the database as installed
func (d *Database) MarkAsInstalled() {
	now := time.Now()
	d.InstalledAt = &now
	d.InstallationFailedAt = nil
}

// MarkAsFailed marks the database installation as failed
func (d *Database) MarkAsFailed() {
	now := time.Now()
	d.InstallationFailedAt = &now
}

// MarkAsUninstalling marks the database as being uninstalled
func (d *Database) MarkAsUninstalling() {
	now := time.Now()
	d.UninstallationRequestedAt = &now
}

// MarkUninstallationFailed marks the uninstallation as failed
func (d *Database) MarkUninstallationFailed() {
	now := time.Now()
	d.UninstallationFailedAt = &now
}
