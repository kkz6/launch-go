package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// TeamInvitation represents a pending team invitation
type TeamInvitation struct {
	ID        string     `gorm:"type:char(26);primaryKey" json:"id"`
	TeamID    string     `gorm:"column:team_id;type:char(26);not null;uniqueIndex:team_invitations_team_id_email_unique,priority:1" json:"team_id"`
	Email     string     `gorm:"type:varchar(255);not null;uniqueIndex:team_invitations_team_id_email_unique,priority:2" json:"email"`
	Role      *string    `gorm:"type:varchar(255)" json:"role,omitempty"`
	CreatedAt *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Team *Team `gorm:"foreignKey:TeamID;references:ID" json:"team,omitempty"`
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
