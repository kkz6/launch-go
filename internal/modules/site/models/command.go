package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Command represents a command executed on a site
type Command struct {
	ID        string              `gorm:"primaryKey;size:26" json:"id"`
	SiteID    string              `gorm:"size:26;not null;index" json:"site_id"`
	UserID    string              `gorm:"size:26;not null;index" json:"user_id"`
	Command   string              `gorm:"type:text;not null" json:"command"`
	Status    enums.CommandStatus `gorm:"size:50;default:'pending'" json:"status"`
	Output    *string             `gorm:"type:text" json:"output,omitempty"`
	ExitCode  *int                `json:"exit_code,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID" json:"site,omitempty"`
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
