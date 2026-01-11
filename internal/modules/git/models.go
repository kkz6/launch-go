package git

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// JSONMap is a helper type for storing JSON data in the database
type JSONMap map[string]interface{}

// Value implements driver.Valuer
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements sql.Scanner
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return nil
	}

	return json.Unmarshal(data, j)
}

// JSONArray is a helper type for storing JSON arrays in the database
type JSONArray []interface{}

// Value implements driver.Valuer
func (j JSONArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements sql.Scanner
func (j *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return nil
	}

	return json.Unmarshal(data, j)
}

// SourceControl represents a connected git provider account/installation
type SourceControl struct {
	ID                      string              `gorm:"primaryKey;size:26" json:"id"`
	UserID                  string              `gorm:"size:26;not null;index" json:"user_id"`
	TeamID                  string              `gorm:"size:26;not null;index" json:"team_id"`
	Provider                GitProviderType     `gorm:"size:50;not null" json:"provider"`
	URL                     *string             `gorm:"size:500" json:"url,omitempty"`
	ProviderID              *string             `gorm:"size:255;index" json:"provider_id,omitempty"`
	ProviderData            JSONMap             `gorm:"type:json" json:"provider_data,omitempty"`
	ProviderAccountID       *string             `gorm:"size:255" json:"provider_account_id,omitempty"`
	Login                   *string             `gorm:"size:255" json:"login,omitempty"`
	Name                    *string             `gorm:"size:255" json:"name,omitempty"`
	Type                    *string             `gorm:"size:50" json:"type,omitempty"`
	AvatarURL               *string             `gorm:"size:500" json:"avatar_url,omitempty"`
	HTMLURL                 *string             `gorm:"size:500" json:"html_url,omitempty"`
	InstallationID          *string             `gorm:"size:255;index" json:"installation_id,omitempty"`
	Permissions             JSONMap             `gorm:"type:json" json:"permissions,omitempty"`
	RepositorySelection     *string             `gorm:"size:50" json:"repository_selection,omitempty"`
	HasMultipleRepositories bool                `gorm:"default:false" json:"has_multiple_repositories"`
	RepositoryCount         int                 `gorm:"default:0" json:"repository_count"`
	ConnectedAt             *time.Time          `json:"connected_at,omitempty"`
	LastSyncedAt            *time.Time          `json:"last_synced_at,omitempty"`
	AdditionalData          JSONMap             `gorm:"type:json" json:"additional_data,omitempty"`
	CreatedAt               time.Time           `json:"created_at"`
	UpdatedAt               time.Time           `json:"updated_at"`
	DeletedAt               gorm.DeletedAt      `gorm:"index" json:"-"`

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
	return ParseAccountType(*s.Type).IsOrganization()
}

// IsUser checks if this source control is for a user account
func (s *SourceControl) IsUser() bool {
	if s.Type == nil {
		return true
	}
	return ParseAccountType(*s.Type).IsUser()
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

// SourceControlRepository represents a repository synced from a git provider
type SourceControlRepository struct {
	ID              string   `gorm:"primaryKey;size:26" json:"id"`
	SourceControlID string   `gorm:"size:26;not null;index" json:"source_control_id"`
	Name            string   `gorm:"size:255;not null" json:"name"`
	FullName        string   `gorm:"size:500;not null;index" json:"full_name"`
	Public          bool     `gorm:"default:true" json:"public"`
	DefaultBranch   *string  `gorm:"size:255" json:"default_branch,omitempty"`
	HTMLURL         *string  `gorm:"size:500" json:"html_url,omitempty"`
	SSHURL          *string  `gorm:"size:500" json:"ssh_url,omitempty"`
	AdditionalData  JSONMap  `gorm:"type:json" json:"additional_data,omitempty"`

	// Relations
	SourceControl *SourceControl `gorm:"foreignKey:SourceControlID" json:"source_control,omitempty"`
}

// TableName returns the table name for SourceControlRepository
func (SourceControlRepository) TableName() string {
	return "source_control_repositories"
}

// BeforeCreate is a GORM hook that generates a ULID before creating a record
func (r *SourceControlRepository) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = utils.NewULID()
	}
	return nil
}

// GetDefaultBranch returns the default branch or "main" if not set
func (r *SourceControlRepository) GetDefaultBranch() string {
	if r.DefaultBranch == nil || *r.DefaultBranch == "" {
		return "main"
	}
	return *r.DefaultBranch
}

// GetHTMLURL returns the HTML URL or empty string if nil
func (r *SourceControlRepository) GetHTMLURL() string {
	if r.HTMLURL == nil {
		return ""
	}
	return *r.HTMLURL
}

// GetSSHURL returns the SSH URL or empty string if nil
func (r *SourceControlRepository) GetSSHURL() string {
	if r.SSHURL == nil {
		return ""
	}
	return *r.SSHURL
}
