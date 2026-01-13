package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Release represents a deployment release for zero-downtime deployments
type Release struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	SiteID     string  `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
	Path       string  `gorm:"type:varchar(500);not null" json:"path"`
	CommitHash *string `gorm:"column:commit_hash;type:varchar(40)" json:"commit_hash,omitempty"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID;references:ID" json:"site,omitempty"`
}

func (Release) TableName() string {
	return "releases"
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
