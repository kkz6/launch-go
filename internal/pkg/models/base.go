package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// BaseModel provides common fields and ULID generation for all models.
// Embed this in your model to get automatic ULID ID generation.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    Name string `gorm:"type:varchar(255)"`
//	}
type BaseModel struct {
	ID        string     `gorm:"type:char(26);primaryKey" json:"id"`
	CreatedAt *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`
}

// BeforeCreate generates a ULID for the ID if not already set
func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = util.NewULID()
	}
	return nil
}

// GetID returns the model's ID. This method enables models embedding
// BaseModel to implement interfaces that require GetID().
func (b *BaseModel) GetID() string {
	return b.ID
}

// InstallableModel provides common fields for models that track installation status.
// Embed this along with BaseModel for resources that can be installed/uninstalled.
//
// Usage:
//
//	type Database struct {
//	    models.BaseModel
//	    models.InstallableModel
//	    ServerID string `gorm:"column:server_id"`
//	    Name     string
//	}
type InstallableModel struct {
	InstalledAt               *time.Time `gorm:"column:installed_at;type:timestamp null" json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `gorm:"column:installation_failed_at;type:timestamp null" json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null" json:"uninstallation_requested_at,omitempty"`
	UninstallationFailedAt    *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null" json:"uninstallation_failed_at,omitempty"`
}

// Status returns the current installation status
func (m *InstallableModel) Status() enumtypes.InstallationStatus {
	if m.IsUninstalling() {
		return enumtypes.StatusUninstalling
	}
	if m.IsFailed() {
		return enumtypes.StatusFailed
	}
	if m.IsInstalled() {
		return enumtypes.StatusInstalled
	}
	return enumtypes.StatusPending
}

// IsInstalled returns true if the resource is installed
func (m *InstallableModel) IsInstalled() bool {
	return m.InstalledAt != nil && m.InstallationFailedAt == nil
}

// IsInstalling returns true if the resource is being installed
func (m *InstallableModel) IsInstalling() bool {
	return m.InstalledAt == nil && m.InstallationFailedAt == nil
}

// IsFailed returns true if the installation failed
func (m *InstallableModel) IsFailed() bool {
	return m.InstallationFailedAt != nil
}

// IsUninstalling returns true if the resource is being uninstalled
func (m *InstallableModel) IsUninstalling() bool {
	return m.UninstallationRequestedAt != nil && m.UninstallationFailedAt == nil
}

// SoftDeleteModel provides soft delete functionality.
// Embed this for models that should be soft-deleted instead of permanently removed.
type SoftDeleteModel struct {
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// ServerScopedModel provides a ServerID field for models scoped to a server.
type ServerScopedModel struct {
	ServerID string `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
}

// TeamScopedModel provides a TeamID field for models scoped to a team.
type TeamScopedModel struct {
	TeamID string `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
}

// UserScopedModel provides a UserID field for models scoped to a user.
type UserScopedModel struct {
	UserID string `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
}

// SiteScopedModel provides a SiteID field for models scoped to a site.
type SiteScopedModel struct {
	SiteID string `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
}

// GetUserHomeDir returns the home directory path for a given user.
// For root and ubuntu users, returns /{user}, otherwise /home/{user}.
func GetUserHomeDir(user string) string {
	if user == "root" || user == "ubuntu" {
		return "/" + user
	}
	return "/home/" + user
}

// GetWorkingDir returns the working directory path for a user with the given working directory name.
// If workingDir is empty, defaults to ".launch".
func GetWorkingDir(user, workingDir string) string {
	if workingDir == "" {
		workingDir = ".launch"
	}
	return GetUserHomeDir(user) + "/" + workingDir
}
