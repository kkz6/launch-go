package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// TeamMember represents the pivot table for team-user relationships
type TeamMember struct {
	ID        string    `gorm:"primaryKey;size:26" json:"id"`
	TeamID    string    `gorm:"size:26;uniqueIndex:idx_team_user;not null" json:"team_id"`
	UserID    string    `gorm:"size:26;uniqueIndex:idx_team_user;not null" json:"user_id"`
	Role      string    `gorm:"size:50;default:'member'" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Team *Team `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name for the TeamMember model
func (tm *TeamMember) TableName() string {
	return "team_members"
}

// BeforeCreate is a GORM hook that sets the ID if not provided
func (tm *TeamMember) BeforeCreate(tx *gorm.DB) error {
	if tm.ID == "" {
		tm.ID = utils.NewULID()
	}

	return nil
}
