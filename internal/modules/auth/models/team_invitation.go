package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// TeamInvitation represents a pending team invitation
type TeamInvitation struct {
	basemodels.BaseModel
	TeamID string  `gorm:"column:team_id;type:char(26);not null;uniqueIndex:team_invitations_team_id_email_unique,priority:1" json:"team_id"`
	Email  string  `gorm:"type:varchar(255);not null;uniqueIndex:team_invitations_team_id_email_unique,priority:2" json:"email"`
	Role   *string `gorm:"type:varchar(255)" json:"role,omitempty"`

	// Relations
	Team *Team `gorm:"foreignKey:TeamID;references:ID" json:"team,omitempty"`
}

// TableName returns the table name for the TeamInvitation model
func (TeamInvitation) TableName() string {
	return "team_invitations"
}
