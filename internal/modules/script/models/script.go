package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Script represents a reusable bash script
type Script struct {
	basemodels.BaseModel
	UserID  string  `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID  *string `gorm:"column:team_id;type:char(26);index" json:"team_id,omitempty"`
	Name    string  `gorm:"type:varchar(255);not null" json:"name"`
	User    string  `gorm:"type:varchar(255);not null;default:root" json:"user"`
	Content string  `gorm:"type:longtext;not null" json:"content"`

	// Relations
	Executions []ScriptExecution `gorm:"foreignKey:ScriptID;references:ID" json:"executions,omitempty"`
}

func (Script) TableName() string {
	return "scripts"
}

// IsTeamShared returns true if the script is shared with a team
func (s *Script) IsTeamShared() bool {
	return s.TeamID != nil && *s.TeamID != ""
}
