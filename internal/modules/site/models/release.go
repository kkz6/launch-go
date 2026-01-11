package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Release represents a deployment release for zero-downtime deployments
type Release struct {
	ID         string         `gorm:"primaryKey;size:26" json:"id"`
	SiteID     string         `gorm:"size:26;not null;index" json:"site_id"`
	Path       string         `gorm:"size:500;not null" json:"path"`
	CommitHash *string        `gorm:"size:40" json:"commit_hash,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID" json:"site,omitempty"`
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
