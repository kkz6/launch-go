package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Redirect represents an HTTP redirect rule for a site
type Redirect struct {
	basemodels.BaseModel
	SiteID string `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
	UserID string `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	Mode   int    `gorm:"type:int;not null" json:"mode"`
	From   string `gorm:"type:text;not null" json:"from"`
	To     string `gorm:"type:text;not null" json:"to"`
	Status string `gorm:"type:varchar(255);not null;default:creating" json:"status"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID;references:ID" json:"site,omitempty"`
}

func (Redirect) TableName() string {
	return "redirects"
}

// IsPermanent returns true if this is a permanent redirect (301)
func (r *Redirect) IsPermanent() bool {
	return r.Mode == 301
}

// IsTemporary returns true if this is a temporary redirect (302)
func (r *Redirect) IsTemporary() bool {
	return r.Mode == 302
}
