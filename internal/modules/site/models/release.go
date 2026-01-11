package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Release represents a deployment release for zero-downtime deployments
type Release struct {
	ID         string         `gorm:"type:char(26);primaryKey" json:"id"`
	SiteID     string         `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
	Path       string         `gorm:"type:varchar(500);not null" json:"path"`
	CommitHash *string        `gorm:"column:commit_hash;type:varchar(40)" json:"commit_hash,omitempty"`
	CreatedAt  *time.Time     `gorm:"type:timestamp null" json:"created_at,omitempty"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID;references:ID" json:"site,omitempty"`
}

func (r *Release) TableName() string {
	return "releases"
}

func (r *Release) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = utils.NewULID()
	}

	return nil
}

// GetShortCommitHash returns the first 7 characters of the commit hash
func (r *Release) GetShortCommitHash() string {
	if r.CommitHash == nil {
		return ""
	}

	if len(*r.CommitHash) < 7 {
		return *r.CommitHash
	}

	return (*r.CommitHash)[:7]
}
