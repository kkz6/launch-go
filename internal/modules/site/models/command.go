package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Command represents a command executed on a site
type Command struct {
	ID        string              `gorm:"type:char(26);primaryKey" json:"id"`
	SiteID    string              `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
	UserID    string              `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	Command   string              `gorm:"type:varchar(255);not null" json:"command"`
	Status    enums.CommandStatus `gorm:"type:varchar(255);not null" json:"status"`
	Output    *string             `gorm:"type:longtext" json:"output,omitempty"`
	ExitCode  *int                `gorm:"column:exit_code" json:"exit_code,omitempty"`
	CreatedAt *time.Time          `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt *time.Time          `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID;references:ID" json:"site,omitempty"`
}

func (c *Command) TableName() string {
	return "commands"
}

func (c *Command) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = utils.NewULID()
	}

	return nil
}

// IsSuccessful returns true if the command completed successfully
func (c *Command) IsSuccessful() bool {
	return c.ExitCode != nil && *c.ExitCode == 0
}
