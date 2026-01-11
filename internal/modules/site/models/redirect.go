package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Redirect represents an HTTP redirect rule for a site
type Redirect struct {
	ID        string             `gorm:"primaryKey;size:26" json:"id"`
	SiteID    string             `gorm:"size:26;not null;index" json:"site_id"`
	UserID    string             `gorm:"size:26;not null;index" json:"user_id"`
	Mode      enums.RedirectMode `gorm:"not null" json:"mode"`
	From      string             `gorm:"size:500;not null" json:"from"`
	To        string             `gorm:"size:500;not null" json:"to"`
	Status    string             `gorm:"size:50" json:"status"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
	DeletedAt gorm.DeletedAt     `gorm:"index" json:"-"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID" json:"site,omitempty"`
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
