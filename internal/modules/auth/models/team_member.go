package models

import (
	"time"
)

// TeamMember represents the pivot table for team-user relationships
type TeamMember struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TeamID    string    `gorm:"size:26;uniqueIndex:team_user_team_id_user_id_unique,priority:1;not null" json:"team_id"`
	UserID    string    `gorm:"size:26;uniqueIndex:team_user_team_id_user_id_unique,priority:2;not null" json:"user_id"`
	Role      *string   `gorm:"size:255" json:"role,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Team *Team `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name for the TeamMember model
func (tm *TeamMember) TableName() string {
	return "team_user"
}
