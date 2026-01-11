package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// SourceControl represents a connected git provider account/installation
type SourceControl struct {
	ID                      string                `gorm:"primaryKey;size:26" json:"id"`
	UserID                  string                `gorm:"size:26;not null;index" json:"user_id"`
	TeamID                  string                `gorm:"size:26;not null;index" json:"team_id"`
	Provider                enums.GitProviderType `gorm:"size:50;not null" json:"provider"`
	URL                     *string               `gorm:"size:500" json:"url,omitempty"`
	ProviderID              *string               `gorm:"size:255;index" json:"provider_id,omitempty"`
	ProviderData            JSONMap               `gorm:"type:json" json:"provider_data,omitempty"`
	ProviderAccountID       *string               `gorm:"size:255" json:"provider_account_id,omitempty"`
	Login                   *string               `gorm:"size:255" json:"login,omitempty"`
	Name                    *string               `gorm:"size:255" json:"name,omitempty"`
	Type                    *string               `gorm:"size:50" json:"type,omitempty"`
	AvatarURL               *string               `gorm:"size:500" json:"avatar_url,omitempty"`
	HTMLURL                 *string               `gorm:"size:500" json:"html_url,omitempty"`
	InstallationID          *string               `gorm:"size:255;index" json:"installation_id,omitempty"`
	Permissions             JSONMap               `gorm:"type:json" json:"permissions,omitempty"`
	RepositorySelection     *string               `gorm:"size:50" json:"repository_selection,omitempty"`
	HasMultipleRepositories bool                  `gorm:"default:false" json:"has_multiple_repositories"`
	RepositoryCount         int                   `gorm:"default:0" json:"repository_count"`
	ConnectedAt             *time.Time            `json:"connected_at,omitempty"`
	LastSyncedAt            *time.Time            `json:"last_synced_at,omitempty"`
	AdditionalData          JSONMap               `gorm:"type:json" json:"additional_data,omitempty"`
	CreatedAt               time.Time             `json:"created_at"`
	UpdatedAt               time.Time             `json:"updated_at"`
	DeletedAt               gorm.DeletedAt        `gorm:"index" json:"-"`

	// Relations
	Repositories []SourceControlRepository `gorm:"foreignKey:SourceControlID" json:"repositories,omitempty"`
}

// TableName returns the table name for SourceControl
func (SourceControl) TableName() string {
	return "source_controls"
}

// BeforeCreate is a GORM hook that generates a ULID before creating a record
func (s *SourceControl) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = utils.NewULID()
	}

	return nil
}

// IsOrganization checks if this source control is for an organization
func (s *SourceControl) IsOrganization() bool {
	if s.Type == nil {
		return false
	}

	return enums.ParseAccountType(*s.Type).IsOrganization()
}

// IsUser checks if this source control is for a user account
func (s *SourceControl) IsUser() bool {
	if s.Type == nil {
		return true
	}

	return enums.ParseAccountType(*s.Type).IsUser()
}

// NeedsSynchronization checks if the source control needs to be synced
func (s *SourceControl) NeedsSynchronization() bool {
	if s.LastSyncedAt == nil {
		return true
	}

	return s.LastSyncedAt.Before(time.Now().Add(-time.Hour))
}

// GetLogin returns the login or empty string if nil
func (s *SourceControl) GetLogin() string {
	if s.Login == nil {
		return ""
	}

	return *s.Login
}

// GetInstallationID returns the installation ID or empty string if nil
func (s *SourceControl) GetInstallationID() string {
	if s.InstallationID == nil {
		return ""
	}

	return *s.InstallationID
}
