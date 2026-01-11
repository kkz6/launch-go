package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// SourceControl represents a connected git provider account/installation
type SourceControl struct {
	ID                      string                `gorm:"type:char(26);primaryKey" json:"id"`
	UserID                  string                `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID                  *string               `gorm:"column:team_id;type:char(26);index" json:"team_id,omitempty"`
	ProviderID              string                `gorm:"column:provider_id;type:varchar(255);not null;index" json:"provider_id"`
	ProviderAccountID       *string               `gorm:"column:provider_account_id;type:varchar(255)" json:"provider_account_id,omitempty"`
	Login                   *string               `gorm:"type:varchar(255)" json:"login,omitempty"`
	Name                    *string               `gorm:"type:varchar(255)" json:"name,omitempty"`
	Type                    *string               `gorm:"type:varchar(255)" json:"type,omitempty"`
	AvatarURL               *string               `gorm:"column:avatar_url;type:varchar(255)" json:"avatar_url,omitempty"`
	HTMLURL                 *string               `gorm:"column:html_url;type:varchar(255)" json:"html_url,omitempty"`
	InstallationID          *string               `gorm:"column:installation_id;type:varchar(255)" json:"installation_id,omitempty"`
	Permissions             *string               `gorm:"type:json" json:"permissions,omitempty"`
	RepositorySelection     *string               `gorm:"column:repository_selection;type:varchar(255)" json:"repository_selection,omitempty"`
	HasMultipleRepositories bool                  `gorm:"column:has_multiple_repositories;default:false" json:"has_multiple_repositories"`
	RepositoryCount         *int                  `gorm:"column:repository_count" json:"repository_count,omitempty"`
	ConnectedAt             *time.Time            `gorm:"column:connected_at;type:timestamp null" json:"connected_at,omitempty"`
	LastSyncedAt            *time.Time            `gorm:"column:last_synced_at;type:timestamp null" json:"last_synced_at,omitempty"`
	AdditionalData          *string               `gorm:"column:additional_data;type:json" json:"additional_data,omitempty"`
	Provider                enums.GitProviderType `gorm:"type:varchar(255);not null;index" json:"provider"`
	URL                     *string               `gorm:"type:varchar(255)" json:"url,omitempty"`
	ProviderData            *string               `gorm:"column:provider_data;type:json" json:"provider_data,omitempty"`
	TokenExpiresAt          *time.Time            `gorm:"column:token_expires_at;type:timestamp null" json:"token_expires_at,omitempty"`
	CreatedAt               *time.Time            `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt               *time.Time            `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Repositories []SourceControlRepository `gorm:"foreignKey:SourceControlID;references:ID" json:"repositories,omitempty"`
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
