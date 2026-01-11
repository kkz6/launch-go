package models

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// SourceControlRepository represents a repository synced from a git provider
type SourceControlRepository struct {
	ID              string  `gorm:"primaryKey;size:26" json:"id"`
	SourceControlID string  `gorm:"size:26;not null;index" json:"source_control_id"`
	Name            string  `gorm:"size:255;not null" json:"name"`
	FullName        string  `gorm:"size:500;not null;index" json:"full_name"`
	Public          bool    `gorm:"default:true" json:"public"`
	DefaultBranch   *string `gorm:"size:255" json:"default_branch,omitempty"`
	HTMLURL         *string `gorm:"size:500" json:"html_url,omitempty"`
	SSHURL          *string `gorm:"size:500" json:"ssh_url,omitempty"`
	AdditionalData  JSONMap `gorm:"type:json" json:"additional_data,omitempty"`

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
