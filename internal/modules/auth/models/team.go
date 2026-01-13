package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Team represents a team/organization in the system
type Team struct {
	basemodels.BaseModel
	UserID               string  `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	Name                 string  `gorm:"type:varchar(255);not null" json:"name"`
	ImagePath            *string `gorm:"column:image_path;type:varchar(255)" json:"image_path,omitempty"`
	PersonalTeam         bool    `gorm:"type:tinyint(1);not null;default:0" json:"personal_team"`
	RequiresSubscription bool    `gorm:"type:tinyint(1);not null;default:1" json:"requires_subscription"`

	// Relations
	Owner       *User            `gorm:"foreignKey:UserID;references:ID" json:"owner,omitempty"`
	Members     []User           `gorm:"many2many:team_user;" json:"members,omitempty"`
	Invitations []TeamInvitation `gorm:"foreignKey:TeamID;references:ID" json:"invitations,omitempty"`
}

// TableName returns the table name for the Team model
func (Team) TableName() string {
	return "teams"
}

// ImageURL returns the team's image URL or a default avatar
func (t *Team) ImageURL() string {
	if t.ImagePath != nil && *t.ImagePath != "" {
		return *t.ImagePath
	}

	return t.DefaultImageURL()
}

// DefaultImageURL generates a default avatar URL using UI Avatars
func (t *Team) DefaultImageURL() string {
	name := t.Name
	if name == "" {
		name = "T"
	}

	return "https://ui-avatars.com/api/?name=" + name + "&color=7F9CF5&background=EBF4FF"
}

// HasUser checks if a user belongs to the team
func (t *Team) HasUser(user *User) bool {
	if user.ID == t.UserID {
		return true
	}

	for _, member := range t.Members {
		if member.ID == user.ID {
			return true
		}
	}

	return false
}

// GetUserID returns the team owner's user ID (implements middleware.TeamInfo)
func (t *Team) GetUserID() string {
	return t.UserID
}
