package models

import (
	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Command represents a command executed on a site
type Command struct {
	basemodels.BaseModel
	basemodels.SiteScoped
	basemodels.TeamScoped
	basemodels.UserScoped
	Command  string                  `gorm:"type:varchar(255);not null" json:"command"`
	Status   sitetypes.CommandStatus `gorm:"type:varchar(255);not null" json:"status"`
	Output   *string                 `gorm:"type:longtext" json:"output,omitempty"`
	ExitCode *int                    `gorm:"column:exit_code" json:"exit_code,omitempty"`

	// Relations
	Site *Site            `gorm:"foreignKey:SiteID;references:ID" json:"site,omitempty"`
	User *authmodels.User `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
}

func (Command) TableName() string {
	return "commands"
}

// IsSuccessful returns true if the command completed successfully
func (c *Command) IsSuccessful() bool {
	return c.ExitCode != nil && *c.ExitCode == 0
}
