package models

import (
	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Command represents a command executed on a site
type Command struct {
	basemodels.BaseModel
	SiteID   string              `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
	TeamID   string              `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	UserID   string              `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	Command  string              `gorm:"type:varchar(255);not null" json:"command"`
	Status   enums.CommandStatus `gorm:"type:varchar(255);not null" json:"status"`
	Output   *string             `gorm:"type:longtext" json:"output,omitempty"`
	ExitCode *int                `gorm:"column:exit_code" json:"exit_code,omitempty"`

	// Relations
	Site *Site              `gorm:"foreignKey:SiteID;references:ID" json:"site,omitempty"`
	User *authmodels.User   `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
}

func (Command) TableName() string {
	return "commands"
}

// IsSuccessful returns true if the command completed successfully
func (c *Command) IsSuccessful() bool {
	return c.ExitCode != nil && *c.ExitCode == 0
}
