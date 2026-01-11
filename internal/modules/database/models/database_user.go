package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Compile-time check that DatabaseUser uses gorm.DB in BeforeCreate
var _ = (*gorm.DB)(nil)

// DatabaseUser represents a database user on a server
type DatabaseUser struct {
	ID                        string     `gorm:"type:char(26);primaryKey" json:"id"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	Name                      string     `gorm:"type:varchar(255);not null" json:"name"`
	Password                  *string    `gorm:"type:longtext" json:"-"`
	Host                      string     `gorm:"type:varchar(255);not null;default:localhost" json:"host"`
	InstalledAt               *time.Time `gorm:"column:installed_at;type:timestamp null" json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `gorm:"column:installation_failed_at;type:timestamp null" json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null" json:"uninstallation_requested_at,omitempty"`
	UninstallationFailedAt    *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null" json:"uninstallation_failed_at,omitempty"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Databases []Database `gorm:"many2many:database_database_user;" json:"databases,omitempty"`
}

func (DatabaseUser) TableName() string {
	return "database_users"
}

func (u *DatabaseUser) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = utils.NewULID()
	}

	if u.Host == "" {
		u.Host = "localhost"
	}

	return nil
}

// Status returns the current installation status
func (u *DatabaseUser) Status() enums.InstallationStatus {
	if u.UninstallationRequestedAt != nil && u.UninstallationFailedAt == nil {
		return enums.StatusUninstalling
	}

	if u.InstallationFailedAt != nil {
		return enums.StatusFailed
	}

	if u.InstalledAt != nil {
		return enums.StatusInstalled
	}

	return enums.StatusPending
}

// IsInstalled returns true if the user is installed
func (u *DatabaseUser) IsInstalled() bool {
	return u.InstalledAt != nil && u.InstallationFailedAt == nil
}

// IsInstalling returns true if the user is being installed
func (u *DatabaseUser) IsInstalling() bool {
	return u.InstalledAt == nil && u.InstallationFailedAt == nil
}

// IsFailed returns true if the installation failed
func (u *DatabaseUser) IsFailed() bool {
	return u.InstallationFailedAt != nil
}

// IsUninstalling returns true if the user is being uninstalled
func (u *DatabaseUser) IsUninstalling() bool {
	return u.UninstallationRequestedAt != nil && u.UninstallationFailedAt == nil
}

// MarkAsInstalled marks the user as installed
func (u *DatabaseUser) MarkAsInstalled() {
	now := time.Now()
	u.InstalledAt = &now
	u.InstallationFailedAt = nil
}

// MarkAsFailed marks the user installation as failed
func (u *DatabaseUser) MarkAsFailed() {
	now := time.Now()
	u.InstallationFailedAt = &now
}

// MarkAsUninstalling marks the user as being uninstalled
func (u *DatabaseUser) MarkAsUninstalling() {
	now := time.Now()
	u.UninstallationRequestedAt = &now
}

// MarkUninstallationFailed marks the uninstallation as failed
func (u *DatabaseUser) MarkUninstallationFailed() {
	now := time.Now()
	u.UninstallationFailedAt = &now
}
