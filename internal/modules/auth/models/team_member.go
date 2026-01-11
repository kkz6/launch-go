package models

import (
	"time"
)

// TeamMember represents the pivot table for team-user relationships
type TeamMember struct {
	ID        uint64     `gorm:"type:bigint unsigned;primaryKey;autoIncrement" json:"id"`
	TeamID    string     `gorm:"column:team_id;type:char(26);not null;uniqueIndex:team_user_team_id_user_id_unique,priority:1" json:"team_id"`
	UserID    string     `gorm:"column:user_id;type:char(26);not null;uniqueIndex:team_user_team_id_user_id_unique,priority:2" json:"user_id"`
	Role      *string    `gorm:"type:varchar(255)" json:"role,omitempty"`
	CreatedAt *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Team *Team `gorm:"foreignKey:TeamID;references:ID" json:"team,omitempty"`
	User *User `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
}

// TableName returns the table name for the TeamMember model
func (tm *TeamMember) TableName() string {
	return "team_user"
}

// GetRole returns the team member's role (implements middleware.TeamMemberInfo)
func (tm *TeamMember) GetRole() string {
	if tm.Role == nil {
		return ""
	}
	return *tm.Role
}

// IsAdmin returns true if the team member has admin role (implements middleware.TeamMemberInfo)
func (tm *TeamMember) IsAdmin() bool {
	return tm.Role != nil && *tm.Role == "admin"
}
