package database

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// InstallationStatus represents the installation state of a database or database user
type InstallationStatus string

const (
	StatusPending      InstallationStatus = "pending"
	StatusInstalling   InstallationStatus = "installing"
	StatusInstalled    InstallationStatus = "installed"
	StatusFailed       InstallationStatus = "failed"
	StatusUninstalling InstallationStatus = "uninstalling"
)

// Database represents a database on a server
type Database struct {
	ID                        string          `gorm:"primaryKey;size:26" json:"id"`
	ServerID                  string          `gorm:"size:26;not null;index" json:"server_id"`
	Name                      string          `gorm:"size:255;not null" json:"name"`
	InstalledAt               *time.Time      `json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time      `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time      `json:"uninstallation_requested_at,omitempty"`
	UninstallationFailedAt    *time.Time      `json:"uninstallation_failed_at,omitempty"`
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
	DeletedAt                 gorm.DeletedAt  `gorm:"index" json:"-"`

	// Relations
	Users []DatabaseUser `gorm:"many2many:database_database_user;" json:"users,omitempty"`
}

func (d *Database) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = utils.NewULID()
	}

	return nil
}

// Status returns the current installation status
func (d *Database) Status() InstallationStatus {
	if d.UninstallationRequestedAt != nil && d.UninstallationFailedAt == nil {
		return StatusUninstalling
	}

	if d.InstallationFailedAt != nil {
		return StatusFailed
	}

	if d.InstalledAt != nil {
		return StatusInstalled
	}

	return StatusPending
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

// DatabaseUser represents a database user on a server
type DatabaseUser struct {
	ID                        string          `gorm:"primaryKey;size:26" json:"id"`
	ServerID                  string          `gorm:"size:26;not null;index" json:"server_id"`
	Name                      string          `gorm:"size:255;not null" json:"name"`
	Password                  string          `gorm:"type:text" json:"-"`
	Host                      string          `gorm:"size:255;default:localhost" json:"host"`
	InstalledAt               *time.Time      `json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time      `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time      `json:"uninstallation_requested_at,omitempty"`
	UninstallationFailedAt    *time.Time      `json:"uninstallation_failed_at,omitempty"`
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
	DeletedAt                 gorm.DeletedAt  `gorm:"index" json:"-"`

	// Relations
	Databases []Database `gorm:"many2many:database_database_user;" json:"databases,omitempty"`
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
func (u *DatabaseUser) Status() InstallationStatus {
	if u.UninstallationRequestedAt != nil && u.UninstallationFailedAt == nil {
		return StatusUninstalling
	}

	if u.InstallationFailedAt != nil {
		return StatusFailed
	}

	if u.InstalledAt != nil {
		return StatusInstalled
	}

	return StatusPending
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

// DatabaseDatabaseUser represents the many-to-many relationship between Database and DatabaseUser
type DatabaseDatabaseUser struct {
	DatabaseID     string    `gorm:"primaryKey;size:26" json:"database_id"`
	DatabaseUserID string    `gorm:"primaryKey;size:26" json:"database_user_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (DatabaseDatabaseUser) TableName() string {
	return "database_database_user"
}
