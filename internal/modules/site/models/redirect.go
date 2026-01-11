package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Redirect represents an HTTP redirect rule for a site
type Redirect struct {
	ID        string     `gorm:"type:char(26);primaryKey" json:"id"`
	SiteID    string     `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
	UserID    string     `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	Mode      int        `gorm:"type:int;not null" json:"mode"`
	From      string     `gorm:"type:text;not null" json:"from"`
	To        string     `gorm:"type:text;not null" json:"to"`
	Status    string     `gorm:"type:varchar(255);not null;default:creating" json:"status"`
	CreatedAt *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID;references:ID" json:"site,omitempty"`
}

func (r *Redirect) TableName() string {
	return "redirects"
}

func (r *Redirect) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = utils.NewULID()
	}

	return nil
}

// IsPermanent returns true if this is a permanent redirect (301)
func (r *Redirect) IsPermanent() bool {
	return r.Mode == 301
}

// IsTemporary returns true if this is a temporary redirect (302)
func (r *Redirect) IsTemporary() bool {
	return r.Mode == 302
}
