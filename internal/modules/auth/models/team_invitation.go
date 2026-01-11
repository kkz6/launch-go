package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// TeamInvitation represents a pending team invitation
type TeamInvitation struct {
	ID        string    `gorm:"primaryKey;size:26" json:"id"`
	TeamID    string    `gorm:"size:26;not null;index" json:"team_id"`
	Email     string    `gorm:"size:255;not null" json:"email"`
	Role      string    `gorm:"size:50;default:'member'" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Team *Team `gorm:"foreignKey:TeamID" json:"team,omitempty"`
}

// TableName returns the table name for the TeamInvitation model
func (ti *TeamInvitation) TableName() string {
	return "team_invitations"
}

// BeforeCreate is a GORM hook that sets the ID if not provided
func (ti *TeamInvitation) BeforeCreate(tx *gorm.DB) error {
	if ti.ID == "" {
		ti.ID = utils.NewULID()
	}

	return nil
}
